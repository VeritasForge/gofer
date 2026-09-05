package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// EventMsg 는 Event 를 bubbletea 메시지로 감싼 것이다.
type EventMsg Event

// tickMsg 는 1초마다 경과 시간을 다시 그리기 위한 신호다.
type tickMsg time.Time

type rowPhase int

const (
	rowWaiting rowPhase = iota
	rowRunning
	rowDone
)

type row struct {
	phase   rowPhase
	started time.Time
	label   string
	detail  string
	outcome Outcome
	elapsed time.Duration
	cost    float64
}

var (
	styleOK     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleFailed = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleDim    = lipgloss.NewStyle().Faint(true)
)

// Model 은 "항목마다 한 줄 + 요약 줄" 화면이다 (설계 6절).
type Model struct {
	title  string
	items  []Item
	labels Labels
	now    func() time.Time
	start  time.Time
	rows   []row
	tally  tally
	sp     spinner.Model
	done   bool
	nameW  int
	subW   int
}

// NewModel 은 모든 항목이 waiting 인 화면을 만든다.
func NewModel(title string, items []Item, labels Labels, now func() time.Time) Model {
	m := Model{
		title: title, items: items, labels: labels, now: now, start: now(),
		rows: make([]row, len(items)),
		sp:   spinner.New(spinner.WithSpinner(spinner.MiniDot)),
	}
	for _, it := range items {
		m.nameW, m.subW = max(m.nameW, len(it.Name)), max(m.subW, len(it.Sub))
	}
	return m
}

func (m Model) Init() tea.Cmd { return tea.Batch(m.sp.Tick, tick()) }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Interrupt
		}
	case EventMsg:
		m.Apply(Event(msg))
		if m.done {
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.sp, cmd = m.sp.Update(msg)
		return m, cmd
	case tickMsg:
		return m, tick()
	}
	return m, nil
}

func (m Model) View() tea.View { return tea.NewView(m.Render()) }

// Done 은 KindAllDone 을 받았는지다.
func (m Model) Done() bool { return m.done }

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// Apply 는 Event 하나를 상태에 반영한다.
func (m *Model) Apply(ev Event) {
	m.tally.add(ev)
	if ev.Kind == KindAllDone {
		m.done = true
		return
	}
	r := &m.rows[ev.Index]
	switch ev.Kind {
	case KindStart:
		*r = row{phase: rowRunning, started: m.now()}
	case KindStage:
		r.label = ev.Label
		if ev.Detail != "" {
			r.detail = ev.Detail
		}
	case KindDone:
		r.phase, r.outcome, r.label, r.detail = rowDone, ev.Outcome, ev.Label, ev.Detail
		r.elapsed, r.cost = ev.Elapsed, ev.CostUSD
	}
}

// Render 는 화면 문자열이다. 색은 기호에만 입혀 테스트에서 ANSI 를 벗기기 쉽게 한다.
func (m Model) Render() string {
	var b strings.Builder
	fmt.Fprintf(&b, " %s · %s\n\n", m.title, m.start.Format("15:04:05"))
	for i, it := range m.items {
		r := m.rows[i]
		var sym string
		var elapsed time.Duration
		label, detail, cost := r.label, r.detail, r.cost
		switch r.phase {
		case rowWaiting:
			sym, label, detail, cost = styleDim.Render("·"), "waiting", "", 0
		case rowRunning:
			sym, elapsed, cost = m.sp.View(), m.now().Sub(r.started), 0
		case rowDone:
			sym, elapsed = symbolStyle(r.outcome).Render(r.outcome.Symbol()), r.elapsed
		}
		b.WriteString(m.line(sym, it, detail, label, elapsed, cost) + "\n")
	}
	b.WriteString("\n " + strings.Repeat("─", 65) + "\n")
	summary := m.tally.summary(m.labels)
	if !m.done {
		summary += " so far"
	}
	b.WriteString(" " + summary + "\n")
	return b.String()
}

func symbolStyle(o Outcome) lipgloss.Style {
	switch o {
	case OutcomeOK:
		return styleOK
	case OutcomeFailed:
		return styleFailed
	default:
		return styleDim
	}
}

// line 은 한 항목의 줄이다: 기호 · 이름 · 부제 · [detail] · [label] · [경과] · [비용].
func (m Model) line(sym string, it Item, detail, label string, elapsed time.Duration, cost float64) string {
	var parts []string
	if detail != "" {
		parts = append(parts, fmt.Sprintf("%-12s", detail))
	}
	if label != "" {
		parts = append(parts, fmt.Sprintf("%-16s", label))
	}
	if e := elapsedText(elapsed); e != "" {
		parts = append(parts, fmt.Sprintf("%6s", e))
	}
	if c := costText(cost); c != "" {
		parts = append(parts, c)
	}
	s := fmt.Sprintf(" %s %-*s  %-*s  %s", sym, m.nameW, it.Name, m.subW, it.Sub, strings.Join(parts, "  "))
	return strings.TrimRight(s, " ")
}
