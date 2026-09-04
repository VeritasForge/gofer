package tui

import (
	"fmt"
	"strings"
	"time"
)

// Summary 는 요약 문구다. 예: "2 updated · 1 up to date · 0 skipped · 1 failed · $2.54".
func Summary(l Labels, ok, noop, skipped, failed int, cost float64) string {
	return fmt.Sprintf("%d %s · %d %s · %d %s · %d %s · $%.2f", ok, l.OK, noop, l.Noop, skipped, l.Skipped, failed, l.Failed, cost)
}

// tally 는 완료 Event 를 세어 요약 문구를 만든다.
type tally struct {
	counts [4]int
	cost   float64
}

func (t *tally) add(ev Event) {
	if ev.Kind == KindDone {
		t.counts[ev.Outcome]++
		t.cost += ev.CostUSD
	}
}

func (t tally) summary(l Labels) string {
	return Summary(l, t.counts[OutcomeOK], t.counts[OutcomeNoop], t.counts[OutcomeSkipped], t.counts[OutcomeFailed], t.cost)
}

func elapsedText(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	return d.Round(time.Second).String()
}

func costText(c float64) string {
	if c <= 0 {
		return ""
	}
	return fmt.Sprintf("$%.2f", c)
}

// joinNonEmpty 는 빈 조각을 빼고 sep 로 잇는다.
func joinNonEmpty(sep string, parts ...string) string {
	var kept []string
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, sep)
}
