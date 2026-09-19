package holiday

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// sampleICS 는 실제 피드에서 잘라낸 표본이다. 줄바꿈은 실제와 같은 CRLF 다.
// 실제 피드는 연휴를 하루씩 나눠 싣지만, 여러 날에 걸친 항목도 바르게 펼치는지 보려고
// 설날 연휴를 사흘짜리 항목 하나로 넣었다. 식목일 항목은 DESCRIPTION 이 접혀 있다.
var sampleICS = strings.Join([]string{
	"BEGIN:VCALENDAR",
	"PRODID:-//Google Inc//Google Calendar 70.9054//EN",
	"BEGIN:VEVENT",
	"DTSTART;VALUE=DATE:20260301",
	"DTEND;VALUE=DATE:20260302",
	"DESCRIPTION:공휴일",
	"SUMMARY:삼일절",
	"END:VEVENT",
	"BEGIN:VEVENT",
	"DTSTART;VALUE=DATE:20260216",
	"DTEND;VALUE=DATE:20260219",
	"DESCRIPTION:공휴일",
	"SUMMARY:설날 연휴",
	"END:VEVENT",
	"BEGIN:VEVENT",
	"DTSTART;VALUE=DATE:20260405",
	"DTEND;VALUE=DATE:20260406",
	"DESCRIPTION:기념일\\n기념일을 숨기려면 Google Calendar 설정 > 대한민",
	" 국의 휴일 캘린더로 이동하세요.",
	"SUMMARY:식목일",
	"END:VEVENT",
	"END:VCALENDAR",
	"",
}, "\r\n")

func TestParseICS(t *testing.T) {
	got := ParseICS([]byte(sampleICS))
	want := []Entry{
		{Date: "2026-02-16", Name: "설날 연휴"},
		{Date: "2026-02-17", Name: "설날 연휴"},
		{Date: "2026-02-18", Name: "설날 연휴"},
		{Date: "2026-03-01", Name: "삼일절"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestParseICSDropsObservances 는 기념일이 걸러지는지 다시 확인한다 — DESCRIPTION 이
// 접혀 들어오므로 이어 붙이지 않으면 판정 자체가 실패한다.
func TestParseICSDropsObservances(t *testing.T) {
	for _, e := range ParseICS([]byte(sampleICS)) {
		if e.Name == "식목일" {
			t.Fatal("식목일 is an observance, not a public holiday")
		}
	}
}

func TestFetchAndSync(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(sampleICS))
	}))
	defer srv.Close()

	p := newPaths(t, "url = \""+srv.URL+"\"\n")
	f, err := Sync(t.Context(), p, day(t, "2026-09-19"))
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if f.Covers.From != "2026-01-01" || f.Covers.To != "2026-12-31" {
		t.Errorf("covers should span whole years, got %+v", f.Covers)
	}
	if f.Source != srv.URL || len(f.Holidays) != 4 {
		t.Errorf("saved file: %+v", f)
	}
	again, err := ReadFile(p.Calendar())
	if err != nil || len(again.Holidays) != 4 {
		t.Errorf("stored file: %v %+v", err, again)
	}
}

func TestFetchRejectsEmptyCalendar(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("BEGIN:VCALENDAR\r\nEND:VCALENDAR\r\n"))
	}))
	defer srv.Close()
	if _, err := Fetch(t.Context(), srv.URL); err == nil {
		t.Fatal("an empty calendar must fail so it never overwrites a good list")
	}
}

func TestFetchReportsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()
	if _, err := Fetch(t.Context(), srv.URL); err == nil {
		t.Fatal("404 must be an error")
	}
}

// TestSyncKeepsOldListWhenFetchFails 는 내려받기가 실패해도 이미 저장된 목록을
// 덮어쓰지 않는지 본다.
func TestSyncKeepsOldListWhenFetchFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := newPaths(t, "url = \""+srv.URL+"\"\n")
	if err := WriteFile(p.Calendar(), storeSample); err != nil {
		t.Fatal(err)
	}
	if _, err := Sync(t.Context(), p, day(t, "2026-09-19")); err == nil {
		t.Fatal("Sync should fail")
	}
	kept, err := ReadFile(p.Calendar())
	if err != nil || len(kept.Holidays) != 3 {
		t.Errorf("the old list must stay intact: %v %+v", err, kept)
	}
}

// TestOpenOrRefreshRecoversStaleList 는 저장된 목록이 오늘을 덮지 못할 때 스스로 받아
// 회복하는지 본다. 사람이 sync 를 잊어도 공휴일 판정이 조용히 멎지 않아야 한다.
func TestOpenOrRefreshRecoversStaleList(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.Write([]byte(sampleICS))
	}))
	defer srv.Close()

	p := newPaths(t, "url = \""+srv.URL+"\"\n")
	stale := storeSample
	stale.Covers = Range{From: "2020-01-01", To: "2020-12-31"} // 오늘을 못 덮는 낡은 목록
	if err := WriteFile(p.Calendar(), stale); err != nil {
		t.Fatal(err)
	}
	cal, warning, err := OpenOrRefresh(t.Context(), p, day(t, "2026-03-03"))
	if err != nil {
		t.Fatalf("OpenOrRefresh: %v", err)
	}
	if warning != "" {
		t.Errorf("a successful refresh should leave no warning, got %q", warning)
	}
	if hits != 1 {
		t.Errorf("should download exactly once, got %d", hits)
	}
	if reason, off := cal.Holiday(day(t, "2026-03-01")); !off || reason != "삼일절" {
		t.Errorf("refreshed list should be in use: got (%q, %v)", reason, off)
	}
}

// TestOpenOrRefreshSkipsNetworkWhenFresh 는 목록이 멀쩡하면 네트워크를 아예 쓰지 않는지 본다.
// 매일 아침 돌아가는 경로에 외부 서버 접속을 넣지 않는다는 것이 이 설계의 핵심이다.
func TestOpenOrRefreshSkipsNetworkWhenFresh(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.Write([]byte(sampleICS))
	}))
	defer srv.Close()

	p := newPaths(t, "url = \""+srv.URL+"\"\n")
	if err := WriteFile(p.Calendar(), storeSample); err != nil {
		t.Fatal(err)
	}
	if _, warning, err := OpenOrRefresh(t.Context(), p, day(t, "2026-03-03")); err != nil || warning != "" {
		t.Fatalf("OpenOrRefresh: %v %q", err, warning)
	}
	if hits != 0 {
		t.Errorf("a fresh list must not touch the network, got %d requests", hits)
	}
}

// TestOpenOrRefreshWarnsWhenRecoveryFails 는 회복까지 실패하면 경고가 남고, 그래도
// 주말 판정은 살아 있는지 본다.
func TestOpenOrRefreshWarnsWhenRecoveryFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := newPaths(t, "url = \""+srv.URL+"\"\n")
	cal, warning, err := OpenOrRefresh(t.Context(), p, day(t, "2026-03-02"))
	if err != nil {
		t.Fatalf("a failed recovery must not block the run: %v", err)
	}
	if !strings.Contains(warning, "gofer holiday sync") {
		t.Errorf("warning should point at sync, got %q", warning)
	}
	if cal.IsWorkday(day(t, "2026-03-14")) {
		t.Error("saturday is still a day off")
	}
}
