package tui

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
)

// Options 는 Run 의 입력이다.
type Options struct {
	Title  string
	Items  []Item
	Labels Labels
	Out    io.Writer
	Log    io.Writer
	TTY    bool
	Now    func() time.Time
}

// ErrInterrupted 는 사용자가 ctrl+c 로 화면을 끝냈을 때다.
var ErrInterrupted = errors.New("interrupted by user")

// IsTerminal 은 f 가 문자 장치(터미널)인지 본다. 외부 모듈 없이 판별한다.
func IsTerminal(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

// Run 은 events 가 닫힐 때까지 소비한다. TTY 면 bubbletea 화면, 아니면 Out 에 한 줄씩. Log 에는 항상 한 줄씩 쓴다.
// ctrl+c 면 즉시 ErrInterrupted 를 돌려주고 남은 events 는 백그라운드에서 Log 로 흘린다 — 호출자는 ctx 를 취소하고
// 처리기가 끝나기를 기다린 뒤 Log 를 닫아야 한다.
func Run(ctx context.Context, o Options, events <-chan Event) error {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	var logPlain *Plain
	if o.Log != nil {
		logPlain = &Plain{W: o.Log, Items: o.Items, Labels: o.Labels, Now: now}
	}
	if !o.TTY {
		out := &Plain{W: o.Out, Items: o.Items, Labels: o.Labels, Now: now}
		for ev := range events {
			out.Write(ev)
			if logPlain != nil {
				logPlain.Write(ev)
			}
		}
		return nil
	}

	p := tea.NewProgram(NewModel(o.Title, o.Items, o.Labels, now), tea.WithContext(ctx), tea.WithOutput(o.Out))
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		for ev := range events {
			if logPlain != nil {
				logPlain.Write(ev)
			}
			p.Send(EventMsg(ev)) // 프로그램이 이미 끝났으면 no-op
		}
	}()
	final, err := p.Run()
	if errors.Is(err, tea.ErrInterrupted) {
		return ErrInterrupted
	}
	if err != nil {
		return err
	}
	if m, ok := final.(Model); ok && !m.Done() {
		return ErrInterrupted
	}
	<-drained
	return nil
}
