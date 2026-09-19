// Package holiday 는 어떤 날짜가 쉬는 날인지 판정한다. 도구 공용이다 —
// `gofer holiday` 명령으로 공휴일 목록을 내려받아 저장하고, 다른 도구는 코드로 판정만 부른다.
package holiday

import "time"

const dateLayout = "2006-01-02"

// Entry 는 달력 항목 하나다. Date 는 "2006-01-02" 형식이다.
type Entry struct {
	Date string `json:"date"`
	Name string `json:"name"`
}

// Calendar 는 판정용 달력이다. 공휴일 목록이 비어 있어도 주말은 판정한다.
type Calendar struct {
	holidays map[string]string // "2026-01-01" → "새해첫날"
	from, to string            // 공휴일 목록이 담고 있는 기간. 목록이 없으면 빈 문자열
}

// New 는 항목 목록과 수록 기간으로 달력을 만든다. extra 는 더할 날짜, ignore 는 뺄 날짜다.
// ignore 가 가장 세다 — extra 에 있어도 ignore 에 있으면 일하는 날이다.
func New(entries []Entry, from, to string, extra, ignore []string) *Calendar {
	m := make(map[string]string, len(entries)+len(extra))
	for _, e := range entries {
		m[e.Date] = e.Name
	}
	for _, d := range extra {
		m[d] = "직접 지정"
	}
	for _, d := range ignore {
		delete(m, d)
	}
	return &Calendar{holidays: m, from: from, to: to}
}

// Holiday 는 그날이 쉬는 날이면 이유를 돌려준다: "주말", "삼일절", "쉬는 날 삼일절" 등.
func (c *Calendar) Holiday(t time.Time) (string, bool) {
	// 공휴일 목록을 먼저 확인 — 주말과 겹치는 공휴일도 공휴일 이름으로 돌려준다.
	if name, ok := c.holidays[t.Format(dateLayout)]; ok {
		return name, true
	}
	if wd := t.Weekday(); wd == time.Saturday || wd == time.Sunday {
		return "주말", true
	}
	return "", false
}

// IsWorkday 는 그날이 일하는 날인지 본다.
func (c *Calendar) IsWorkday(t time.Time) bool {
	_, off := c.Holiday(t)
	return !off
}

// Covers 는 그 날짜가 공휴일 목록의 수록 기간 안인지 본다. 범위 밖이면 주말 판정만 믿을 수 있다.
func (c *Calendar) Covers(t time.Time) bool {
	if c.from == "" || c.to == "" {
		return false
	}
	d := t.Format(dateLayout)
	return c.from <= d && d <= c.to
}
