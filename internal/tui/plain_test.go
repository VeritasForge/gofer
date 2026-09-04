package tui

import (
	"bytes"
	"testing"
	"time"
)

var testLabels = Labels{OK: "updated", Noop: "up to date", Skipped: "skipped", Failed: "failed"}

func fixedNow() time.Time { return time.Date(2026, 9, 5, 8, 41, 2, 0, time.UTC) }

func TestPlainWritesOneLinePerEvent(t *testing.T) {
	var buf bytes.Buffer
	p := &Plain{W: &buf, Items: []Item{{"example-api", "main"}, {"example-docs", "main"}}, Labels: testLabels, Now: fixedNow}
	for _, ev := range []Event{
		{Kind: KindStart, Index: 0},
		{Kind: KindStage, Index: 0, Label: "fetch"},
		{Kind: KindStage, Index: 0, Label: "/understand …", Detail: "+13 commits"},
		{Kind: KindDone, Index: 0, Outcome: OutcomeOK, Label: "graph updated", Detail: "+13 commits", Elapsed: 252 * time.Second, CostUSD: 1.83},
		{Kind: KindStart, Index: 1},
		{Kind: KindDone, Index: 1, Outcome: OutcomeNoop, Label: "up to date"},
		{Kind: KindAllDone},
	} {
		p.Write(ev)
	}
	want := `08:41:02 example-api start
08:41:02 example-api fetch
08:41:02 example-api /understand … +13 commits
08:41:02 example-api ✓ graph updated +13 commits 4m12s $1.83
08:41:02 example-docs start
08:41:02 example-docs – up to date
08:41:02 done · 1 updated · 1 up to date · 0 skipped · 0 failed · $1.83
`
	if buf.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), want)
	}
}

func TestSummary(t *testing.T) {
	got := Summary(testLabels, 2, 1, 0, 1, 2.54)
	want := "2 updated · 1 up to date · 0 skipped · 1 failed · $2.54"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestOutcomeSymbols(t *testing.T) {
	if OutcomeOK.Symbol()+OutcomeNoop.Symbol()+OutcomeSkipped.Symbol()+OutcomeFailed.Symbol() != "✓–↷✗" {
		t.Error("symbols changed")
	}
}
