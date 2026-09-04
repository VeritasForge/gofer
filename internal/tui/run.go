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
	Cancel func() // 사용자가 화면을 중단하거나 프로그램이 일찍 끝났을 때 호출 — 처리기가 ctx 를 보고 멈추게 한다. nil 허용
}

// ErrInterrupted 는 사용자가 ctrl+c 로 화면을 끝냈을 때다.
var ErrInterrupted = errors.New("interrupted by user")

// IsTerminal 은 f 가 문자 장치(터미널)인지 본다. 외부 모듈 없이 판별한다.
func IsTerminal(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

// Run 은 어느 경로로 끝나든 events 가 닫히고 다 비워진 뒤에만 돌아온다. TTY 면 bubbletea 화면, 아니면 Out 에
// 한 줄씩. Log 에는 항상 한 줄씩 쓴다. 화면이 일찍 끝나면(ctrl+c, 프로그램 오류) Cancel 을 불러 처리기가 멈추게
// 하고, 그래도 남은 events 를 마저 비운 뒤에야 돌아온다 — 그래야 호출자가 Run 이 끝난 뒤 Log 를 안전하게 닫을 수 있다.
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
	early := err != nil
	if !early {
		m, ok := final.(Model)
		early = !ok || !m.Done()
	}
	if early && o.Cancel != nil {
		o.Cancel()
	}
	<-drained
	if errors.Is(err, tea.ErrInterrupted) {
		return ErrInterrupted
	}
	if err != nil {
		return err
	}
	if early {
		return ErrInterrupted
	}
	return nil
}
