package tui

import (
	"regexp"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func newTestModel(now *time.Time) Model {
	items := []Item{{"example-api", "main"}, {"example-web", "develop"}, {"example-docs", "main"}}
	return NewModel("ua-refresh · 3 repos", items, testLabels, func() time.Time { return *now })
}

func TestRenderWhileRunning(t *testing.T) {
	now := fixedNow()
	m := newTestModel(&now)
	m.Apply(Event{Kind: KindStart, Index: 0})
	m.Apply(Event{Kind: KindDone, Index: 0, Outcome: OutcomeOK, Label: "graph updated", Detail: "+13 commits", Elapsed: 252 * time.Second, CostUSD: 1.83})
	m.Apply(Event{Kind: KindStart, Index: 1})
	m.Apply(Event{Kind: KindStage, Index: 1, Label: "merge", Detail: "+2 commits"})
	m.Apply(Event{Kind: KindStage, Index: 1, Label: "/understand …"}) // Detail 이 비면 이전 값 유지
	now = now.Add(97 * time.Second)
	want := ` ua-refresh · 3 repos · 08:41:02

 ✓ example-api   main     +13 commits   graph updated      4m12s  $1.83
 ⠋ example-web   develop  +2 commits    /understand …      1m37s
 · example-docs  main     waiting

 ─────────────────────────────────────────────────────────────────
 1 updated · 0 up to date · 0 skipped · 0 failed · $1.83 so far
`
	if got := ansiRE.ReplaceAllString(m.Render(), ""); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderWhenDone(t *testing.T) {
	now := fixedNow()
	m := newTestModel(&now)
	m.Apply(Event{Kind: KindStart, Index: 0})
	m.Apply(Event{Kind: KindDone, Index: 0, Outcome: OutcomeNoop, Label: "up to date"})
	m.Apply(Event{Kind: KindStart, Index: 1})
	m.Apply(Event{Kind: KindDone, Index: 1, Outcome: OutcomeSkipped, Label: `skipped: branch is "feature-x", expected "develop"`})
	m.Apply(Event{Kind: KindStart, Index: 2})
	m.Apply(Event{Kind: KindDone, Index: 2, Outcome: OutcomeFailed, Label: "ff-merge failed: 1 local commit ahead of origin", Elapsed: 3 * time.Second})
	m.Apply(Event{Kind: KindAllDone})
	want := ` ua-refresh · 3 repos · 08:41:02

 – example-api   main     up to date
 ↷ example-web   develop  skipped: branch is "feature-x", expected "develop"
 ✗ example-docs  main     ff-merge failed: 1 local commit ahead of origin      3s

 ─────────────────────────────────────────────────────────────────
 0 updated · 1 up to date · 1 skipped · 1 failed · $0.00
`
	if got := ansiRE.ReplaceAllString(m.Render(), ""); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if !m.Done() {
		t.Error("Done() should be true after KindAllDone")
	}
}

func TestUpdateQuitsOnAllDone(t *testing.T) {
	now := fixedNow()
	m := newTestModel(&now)
	next, cmd := m.Update(EventMsg{Kind: KindAllDone})
	if cmd == nil {
		t.Fatal("expected a command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("expected QuitMsg, got %T", cmd())
	}
	if !next.(Model).Done() {
		t.Error("model should be done")
	}
}

func TestUpdateInterruptsOnCtrlC(t *testing.T) {
	now := fixedNow()
	m := newTestModel(&now)
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("expected a command")
	}
	if _, ok := cmd().(tea.InterruptMsg); !ok {
		t.Errorf("expected InterruptMsg, got %T", cmd())
	}
}
