package tui

import (
	"fmt"
	"io"
	"time"
)

// Plain 은 Event 를 한 줄씩 쓴다. 터미널이 아닐 때(launchd)의 화면이고, 터미널일 때도 로그 파일에 같은 내용을 남긴다.
type Plain struct {
	W      io.Writer
	Items  []Item
	Labels Labels
	Now    func() time.Time
	tally  tally
}

// Write 는 Event 하나를 한 줄로 쓴다.
func (p *Plain) Write(ev Event) {
	p.tally.add(ev)
	ts := p.Now().Format("15:04:05")
	if ev.Kind == KindAllDone {
		fmt.Fprintf(p.W, "%s done · %s\n", ts, p.tally.summary(p.Labels))
		return
	}
	var body string
	switch ev.Kind {
	case KindStart:
		body = "start"
	case KindStage:
		body = joinNonEmpty(" ", ev.Label, ev.Detail)
	case KindDone:
		body = joinNonEmpty(" ", ev.Outcome.Symbol()+" "+ev.Label, ev.Detail, elapsedText(ev.Elapsed), costText(ev.CostUSD))
	}
	fmt.Fprintf(p.W, "%s %s %s\n", ts, p.Items[ev.Index].Name, body)
}
