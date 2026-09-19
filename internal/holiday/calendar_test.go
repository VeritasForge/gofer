package holiday

import (
	"testing"
	"time"
)

// day 는 "2026-01-01" 을 그날 0시의 time.Time 으로 바꾼다. 테스트에서 날짜를 고정하는 데 쓴다.
func day(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.ParseInLocation(dateLayout, s, time.Local)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestHolidayReasons(t *testing.T) {
	cal := New([]Entry{
		{Date: "2026-03-01", Name: "삼일절"},
		{Date: "2026-03-02", Name: "쉬는 날 삼일절"},
		{Date: "2026-05-01", Name: "노동절"},
	}, "2026-01-01", "2026-12-31", []string{"2026-10-12"}, []string{"2026-05-01"})

	cases := []struct {
		date   string
		reason string
		off    bool
		why    string
	}{
		{"2026-03-14", "주말", true, "토요일"},
		{"2026-03-15", "주말", true, "일요일"},
		{"2026-03-02", "쉬는 날 삼일절", true, "대체공휴일, 월요일"},
		{"2026-10-12", "직접 지정", true, "extra 로 더한 날, 월요일"},
		{"2026-05-01", "", false, "ignore 로 뺀 날, 금요일"},
		{"2026-03-03", "", false, "평일"},
	}
	for _, c := range cases {
		reason, off := cal.Holiday(day(t, c.date))
		if off != c.off || reason != c.reason {
			t.Errorf("%s (%s): got (%q, %v), want (%q, %v)", c.date, c.why, reason, off, c.reason, c.off)
		}
		if want := !c.off; cal.IsWorkday(day(t, c.date)) != want {
			t.Errorf("%s (%s): IsWorkday should be %v", c.date, c.why, want)
		}
	}
}

// TestIgnoreBeatsExtra 는 같은 날이 양쪽에 있을 때 ignore 가 이기는지 본다.
func TestIgnoreBeatsExtra(t *testing.T) {
	cal := New(nil, "2026-01-01", "2026-12-31", []string{"2026-10-12"}, []string{"2026-10-12"})
	if !cal.IsWorkday(day(t, "2026-10-12")) {
		t.Error("ignore should win over extra")
	}
}

// TestWeekendWithoutList 는 목록이 하나도 없어도 주말은 판정되는지 본다.
func TestWeekendWithoutList(t *testing.T) {
	cal := New(nil, "", "", nil, nil)
	if cal.IsWorkday(day(t, "2026-03-14")) {
		t.Error("saturday is a day off even without a holiday list")
	}
	if !cal.IsWorkday(day(t, "2026-03-02")) {
		t.Error("without a list, a public holiday cannot be detected")
	}
}

func TestCovers(t *testing.T) {
	cal := New(nil, "2026-01-01", "2026-12-31", nil, nil)
	if !cal.Covers(day(t, "2026-12-31")) {
		t.Error("last day of the range should be covered")
	}
	if cal.Covers(day(t, "2027-01-01")) {
		t.Error("a day past the range should not be covered")
	}
	if New(nil, "", "", nil, nil).Covers(day(t, "2026-03-02")) {
		t.Error("an empty range covers nothing")
	}
}
