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

// FormatElapsed 는 화면과 DM 이 공유하는 경과 시간 형식이다: 1분 미만 "37s", 1시간 미만 "4m12s"/"2m05s"
// (초는 0 으로 채움), 1시간 이상 "1h02m05s". d <= 0 이면 빈 문자열.
func FormatElapsed(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	total := int(d.Round(time.Second).Seconds())
	h, rem := total/3600, total%3600
	m, s := rem/60, rem%60
	switch {
	case h > 0:
		return fmt.Sprintf("%dh%02dm%02ds", h, m, s)
	case m > 0:
		return fmt.Sprintf("%dm%02ds", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}

func elapsedText(d time.Duration) string {
	return FormatElapsed(d)
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
