package tui

import (
	"bytes"
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestRunWaitsForDrainOnEarlyExit 는 TTY 화면이 일찍(오류로) 끝나도 Run 이 Cancel 을 부르고
// events 를 마저 다 비운 뒤에야 돌아오는지 확인한다. 이 프로세스에는 실제 조종 터미널이 없어
// bubbletea 의 내부 프로그램이 시작하자마자 오류로 끝난다 — 그 오류 경로가 곧 화면 조기 종료 경로다.
// producer 는 Run 이 Cancel 을 부를 때까지 이벤트를 보내지 않고 기다리므로, Run 이 드레인을 기다리지
// 않고 먼저 돌아온다면 마지막 이벤트는 Log 에 닿지 못한다.
func TestRunWaitsForDrainOnEarlyExit(t *testing.T) {
	events := make(chan Event)
	var cancelCalled atomic.Bool

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// 실제 TTY 가 있는 환경에서 이 테스트가 돌아갈 경우를 대비한 안전장치 — Run 이 곧바로
	// 오류로 끝나지 않더라도 이 취소로 결국 오류 경로를 타게 만들어 테스트가 멈추지 않게 한다.
	time.AfterFunc(2*time.Second, cancel)

	go func() {
		for !cancelCalled.Load() {
			time.Sleep(time.Millisecond)
		}
		events <- Event{Kind: KindAllDone}
		close(events)
	}()

	var out, log bytes.Buffer
	err := Run(ctx, Options{
		Items: nil, Labels: Labels{OK: "updated"},
		Out: &out, Log: &log, TTY: true,
		Cancel: func() { cancelCalled.Store(true) },
	}, events)

	if err == nil {
		t.Fatal("want a non-nil error (no controlling TTY in this test environment)")
	}
	// Run 이 <-drained 를 기다린 뒤에야 돌아왔다면, 드레인 고루틴이 이미 이 줄을 Log 에 썼어야 한다.
	if !strings.Contains(log.String(), "done ·") {
		t.Errorf("Run returned before draining the final event; log:\n%s", log.String())
	}
}
