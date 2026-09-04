// Package tui 는 "항목 N개를 순차 처리하며 상태를 보여주는" 도구 공용 화면이다. 처리기가 보내는 Event 만 소비하며,
// 터미널이면 bubbletea 화면을, 아니면 한 줄 로그를 만든다. 두 화면이 같은 Event 를 읽으므로 어긋날 수 없다.
package tui

import "time"

// Kind 는 Event 의 종류다.
type Kind int

const (
	KindStart   Kind = iota // 항목 처리 시작
	KindStage               // 단계 전환: Label 에 단계명, Detail 은 선택
	KindDone                // 항목 완료
	KindAllDone             // 전체 완료
)

// Outcome 은 항목 하나의 결과다.
type Outcome int

const (
	OutcomeOK      Outcome = iota // 했고 성공
	OutcomeNoop                   // 할 일이 없었음
	OutcomeSkipped                // 조건이 안 맞아 건너뜀
	OutcomeFailed                 // 실패
)

// Symbol 은 한 글자 표시다.
func (o Outcome) Symbol() string {
	switch o {
	case OutcomeOK:
		return "✓"
	case OutcomeNoop:
		return "–"
	case OutcomeSkipped:
		return "↷"
	default:
		return "✗"
	}
}

// Item 은 한 줄에 보일 항목이다.
type Item struct {
	Name string
	Sub  string
}

// Labels 는 요약 줄에 쓸 결과별 문구다.
type Labels struct {
	OK, Noop, Skipped, Failed string
}

// Event 는 처리기가 화면에 알리는 한 사건이다.
type Event struct {
	Kind    Kind
	Index   int
	Outcome Outcome
	Label   string
	Detail  string
	Elapsed time.Duration
	CostUSD float64
}
