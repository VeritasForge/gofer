package holiday

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// ParseICS 는 일정 교환 형식(iCalendar) 본문에서 공휴일 항목만 뽑아 날짜순으로 돌려준다.
// DESCRIPTION 이 "공휴일" 로 시작하는 항목만 남기므로 기념일(식목일·어버이날 등)은 걸러진다.
func ParseICS(body []byte) []Entry {
	var out []Entry
	var start, end, name, desc string
	for _, line := range unfold(body) {
		switch {
		case line == "BEGIN:VEVENT":
			start, end, name, desc = "", "", "", ""
		case strings.HasPrefix(line, "DTSTART;VALUE=DATE:"):
			start = strings.TrimPrefix(line, "DTSTART;VALUE=DATE:")
		case strings.HasPrefix(line, "DTEND;VALUE=DATE:"):
			end = strings.TrimPrefix(line, "DTEND;VALUE=DATE:")
		case strings.HasPrefix(line, "SUMMARY:"):
			name = strings.TrimPrefix(line, "SUMMARY:")
		case strings.HasPrefix(line, "DESCRIPTION:"):
			desc = strings.TrimPrefix(line, "DESCRIPTION:")
		case line == "END:VEVENT":
			if strings.HasPrefix(desc, "공휴일") {
				out = append(out, expand(start, end, name)...)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date < out[j].Date })
	return out
}

// unfold 는 CRLF 를 떼고, 공백이나 탭으로 시작하는 이어진 줄을 앞줄에 붙인다.
// 이 형식은 긴 줄을 여러 줄로 접어 보내므로, 붙이지 않으면 값이 잘린 채로 판정된다.
func unfold(body []byte) []string {
	var out []string
	for _, l := range strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n") {
		l = strings.TrimSuffix(l, "\r")
		if len(out) > 0 && (strings.HasPrefix(l, " ") || strings.HasPrefix(l, "\t")) {
			out[len(out)-1] += l[1:]
			continue
		}
		out = append(out, l)
	}
	return out
}

// expand 는 시작일부터 끝일 직전까지를 하루씩 펼친다 — 이 형식의 끝 날짜는 그날을 포함하지 않는다.
func expand(start, end, name string) []Entry {
	from, err := time.Parse("20060102", start)
	if err != nil {
		return nil
	}
	to, err := time.Parse("20060102", end)
	if err != nil || !to.After(from) {
		to = from.AddDate(0, 0, 1)
	}
	var out []Entry
	for d := from; d.Before(to); d = d.AddDate(0, 0, 1) {
		out = append(out, Entry{Date: d.Format(dateLayout), Name: name})
	}
	return out
}

// Fetch 는 달력을 내려받아 공휴일 항목만 뽑는다. 항목이 하나도 없으면 오류다 —
// 빈 목록으로 저장 파일을 덮어쓰면 판정이 통째로 망가지기 때문이다.
func Fetch(ctx context.Context, rawURL string) ([]Entry, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("holiday calendar: invalid URL: %w", unwrapURLError(err))
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("holiday calendar: %w", unwrapURLError(err))
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("holiday calendar: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("holiday calendar: %w", err)
	}
	entries := ParseICS(body)
	if len(entries) == 0 {
		return nil, errors.New("holiday calendar: no public holidays found; keeping the stored list")
	}
	return entries, nil
}

// unwrapURLError 는 *url.Error 의 URL 을 담지 않은 내부 오류를 꺼낸다.
func unwrapURLError(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

// Sync 는 설정의 주소에서 달력을 받아 저장한다. 받기가 실패하면 저장 파일을 건드리지 않는다.
func Sync(ctx context.Context, p Paths, now time.Time) (File, error) {
	cfg, err := LoadConfig(p.Config)
	if err != nil {
		return File{}, err
	}
	entries, err := Fetch(ctx, cfg.URL)
	if err != nil {
		return File{}, err
	}
	// 수록 기간은 연 단위로 잡는다. 첫·마지막 항목 날짜를 그대로 쓰면 12월 26일부터
	// 연말까지가 "판정 불가" 로 빠져 매년 연말에 헛경고가 난다.
	f := File{
		SyncedAt: now.Format(time.RFC3339),
		Source:   cfg.URL,
		Covers: Range{
			From: entries[0].Date[:4] + "-01-01",
			To:   entries[len(entries)-1].Date[:4] + "-12-31",
		},
		Holidays: entries,
	}
	return f, WriteFile(p.Calendar(), f)
}

// OpenOrRefresh 는 Open 과 같되, 저장된 목록이 now 를 덮지 못하는 바로 그때만
// 한 번 내려받아 본다. 목록이 멀쩡한 동안에는 네트워크를 전혀 쓰지 않는다 —
// 매일 아침 돌아가는 경로에 외부 서버 접속을 두지 않는 것이 이 설계의 핵심이다.
//
// 사람이 sync 를 잊으면 공휴일 판정이 조용히 멎기 때문에 이 회복 경로를 둔다.
// 회복까지 실패해도 그날 실행을 막지 않고, 주말만 판정한 채 경고를 돌려준다.
func OpenOrRefresh(ctx context.Context, p Paths, now time.Time) (*Calendar, string, error) {
	cal, warning, err := Open(p, now)
	if err != nil || warning == "" {
		return cal, warning, err
	}
	if _, serr := Sync(ctx, p, now); serr != nil {
		return cal, fmt.Sprintf("%s (auto refresh failed: %v)", warning, serr), nil
	}
	return Open(p, now)
}
