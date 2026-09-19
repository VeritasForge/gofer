# gofer holiday 구현 플랜 — 쉬는 날에는 그래프를 갱신하지 않는다

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `gofer holiday` 도구를 새로 만들어 한국 공휴일 목록을 내려받아 로컬에 저장하고, `gofer ua-refresh run`이 토요일·일요일·공휴일·대체공휴일에는 아무 알림도 보내지 않고 끝나게 한다.

**Architecture:** 공휴일 목록은 사람이 `gofer holiday sync`를 한 번 실행해 파일로 저장하고, 매일 아침 돌아가는 경로는 그 파일만 읽는다. 네트워크는 `sync` 때만 쓴다. 판정 로직은 `internal/holiday` 공용 패키지에 두고 `ua-refresh`가 코드로 호출한다. 판정할 수 없을 때는 실행을 막지 않고 주말만 걸러 낸 뒤 경고를 남긴다.

**Tech Stack:** Go 1.26, cobra, BurntSushi/toml, 표준 라이브러리 `net/http`·`encoding/json`. 새 의존성 없음.

**Spec:** `docs/superpowers/specs/2026-09-12-holiday-workdays-design.md`

## Global Constraints

스펙과 `CLAUDE.md`에서 그대로 옮긴 제약이다. 모든 Task의 요구사항에 이 절이 암묵적으로 포함된다.

- Go 1.26, 모듈 하나(`module gofer`).
- 외부 의존성은 여섯 개(cobra · fang · bubbletea/v2 · bubbles/v2 · lipgloss/v2 · BurntSushi/toml)로 고정한다. **이번 작업에서 하나도 추가하지 않는다.** `go.mod`의 `require` 줄이 늘면 실패다.
- `cmd/`는 플래그 파싱만 하고 로직은 `internal/`에 둔다.
- 조직 중립: 코드·주석·README·테스트·예제·문서 어디에도 특정 회사·팀·레포·브랜치 정보를 쓰지 않는다. 예제는 `~/src/example-api`, `main`처럼 중립적으로 쓴다.
- 환경에 따라 달라지는 값(경로, 시각, 주소)은 전부 사용자 설정 파일로 뺀다.
- `go test ./...`와 `go vet ./...`가 통과해야 한다.
- **네트워크에 나가는 테스트를 만들지 않는다.** 외부 서버는 `httptest`로 흉내 낸다.
- 주석과 문서는 한국어로 쓴다. **사용자에게 보이는 CLI 출력은 기존 관행대로 영어로 쓴다** — 단, 달력에서 읽어 온 휴일 이름("쉬는 날 삼일절")은 그대로 한국어다. 스펙 6절의 화면 예시는 내용을 보이려고 한국어로 적었으므로, Task 8에서 실제 출력에 맞춰 고친다.

## 전체 완료조건

모든 Task를 마친 뒤 아래를 **실제로 실행해 출력을 확인**한다.

```bash
cd /Users/jaeyoungcho/lab/gofer
go vet ./... && go test ./...          # 통과
go build -o bin/gofer . && ./bin/gofer --help | grep holiday   # holiday 가 도구 목록에 있다
git diff 4a7f9f2 --stat -- go.mod go.sum   # 출력이 비어 있다 (의존성이 늘지 않았다)
```

그리고 손으로 다음을 확인한다.

1. `./bin/gofer holiday sync` → 목록이 저장되고 건수가 출력된다.
2. `./bin/gofer holiday check 2026-03-02` → `holiday — 쉬는 날 삼일절`이 나온다.
3. `./bin/gofer holiday check 2026-03-03` → `workday`가 나온다.
4. `./bin/gofer ua-refresh status` → `workdays only`와 오늘 판정 줄이 보인다.

## 진행 추적

이 파일의 체크박스(`- [ ]`)를 갱신해 진행을 남긴다. Task를 마칠 때마다 그 Task의 완료조건에 적힌 검증 명령을 **실제로 실행하고 출력을 확인한 뒤** 체크한다. 명령을 돌리지 않은 채 통과했다고 적지 않는다.

## 사용할 스킬

- 각 Task 구현: `superpowers:test-driven-development` (테스트 먼저, 실패 확인, 최소 구현, 통과 확인, 커밋)
- Task 실행 방식: `superpowers:subagent-driven-development`(권장) 또는 `superpowers:executing-plans`
- 코드 리뷰: subagent-driven-development의 Task reviewer를 쓰면 그것으로 충분하다. executing-plans나 순수 plan mode로 진행했다면 Task 8에서 `/code-review`를 최소 1회 실행한다.

## File Structure

| 파일 | 책임 | Task |
|---|---|---|
| `internal/holiday/calendar.go` | 날짜 하나가 쉬는 날인지 판정한다. 파일도 네트워크도 모른다 | 1 |
| `internal/holiday/paths.go` | 설정 파일과 저장 파일의 위치 | 2 |
| `internal/holiday/store.go` | 설정 읽기, 저장 파일 읽고 쓰기, 둘을 합쳐 달력 만들기 | 2 |
| `internal/holiday/ics.go` | 달력 내려받기와 일정 교환 형식 해석 | 3 |
| `cmd/holiday/holiday.go` `sync.go` `list.go` `check.go` | 플래그 파싱과 출력 | 4 |
| `cmd/root.go` | `holiday` 도구 등록 (한 줄 추가) | 4 |
| `internal/uarefresh/config.go` | `workdays_only` 설정 항목 | 5 |
| `internal/uarefresh/command.go` | 실행 전 쉬는 날 게이트, 목록이 낡았을 때의 회복 호출 | 6 |
| `internal/uarefresh/result.go` | 회복 실패 경고를 결과에 싣는 필드 | 6 |
| `internal/uarefresh/report.go` | 그 경고를 Slack 본문 끝에 한 줄로 | 6 |
| `cmd/uarefresh/run.go` | `--force` 플래그 | 6 |
| `internal/uarefresh/status.go` | 오늘 판정 표시 | 7 |
| `cmd/uarefresh/status.go` | 달력 경로 전달 | 7 |

---

### Task 1: 날짜 판정

파일도 네트워크도 모르는 순수 로직부터 만든다. 이후 모든 Task가 이 타입 위에 선다.

**Files:**
- Create: `internal/holiday/calendar.go`
- Test: `internal/holiday/calendar_test.go`

**Interfaces:**
- Consumes: 없음
- Produces:
  - `type Entry struct { Date string; Name string }` — `Date`는 `"2006-01-02"` 형식
  - `func New(entries []Entry, from, to string, extra, ignore []string) *Calendar`
  - `func (c *Calendar) Holiday(t time.Time) (reason string, off bool)`
  - `func (c *Calendar) IsWorkday(t time.Time) bool`
  - `func (c *Calendar) Covers(t time.Time) bool`
  - `const dateLayout = "2006-01-02"` (패키지 안에서만 쓴다)

- [ ] **Step 1: 실패하는 테스트 작성**

`internal/holiday/calendar_test.go`:

```go
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
```

- [ ] **Step 2: 테스트가 실패하는지 확인**

Run: `go test ./internal/holiday/ -run 'TestHoliday|TestIgnore|TestWeekend|TestCovers' -v`
Expected: 빌드 실패 — `undefined: New`, `undefined: dateLayout`

- [ ] **Step 3: 최소 구현 작성**

`internal/holiday/calendar.go`:

```go
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
	if wd := t.Weekday(); wd == time.Saturday || wd == time.Sunday {
		return "주말", true
	}
	if name, ok := c.holidays[t.Format(dateLayout)]; ok {
		return name, true
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
```

- [ ] **Step 4: 테스트가 통과하는지 확인**

Run: `go test ./internal/holiday/ -v`
Expected: PASS — 네 개 테스트 모두 통과

- [ ] **Step 5: 커밋**

```bash
git add internal/holiday/calendar.go internal/holiday/calendar_test.go
git commit -m "feat(holiday): date judgement with weekends, extra and ignore

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

**Task 1 완료조건:** `go test ./internal/holiday/ -v`가 PASS이고, `go vet ./internal/holiday/`가 조용하다.

---

### Task 2: 파일 위치, 설정 읽기, 달력 열기

저장된 목록과 사람이 쓴 설정을 합쳐 Task 1의 `Calendar`를 만든다. **판정할 수 없을 때 실행을 막지 않는다는 정책이 이 Task의 `Open` 안에 들어간다** — 부르는 쪽이 매번 다시 정하지 않게 하기 위해서다.

**Files:**
- Create: `internal/holiday/paths.go`, `internal/holiday/store.go`
- Test: `internal/holiday/store_test.go`

**Interfaces:**
- Consumes: Task 1의 `Entry`, `New`, `Calendar`, `dateLayout`
- Produces:
  - `type Paths struct { Config string; StateDir string }`, `func (p Paths) Calendar() string`, `func DefaultPaths() (Paths, error)`
  - `type Config struct { URL string; Extra []string; Ignore []string }`, `func LoadConfig(path string) (Config, error)`
  - `type Range struct { From string; To string }`
  - `type File struct { SyncedAt string; Source string; Covers Range; Holidays []Entry }`
  - `func ReadFile(path string) (File, error)`, `func WriteFile(path string, f File) error`
  - `func Open(p Paths, now time.Time) (*Calendar, string, error)` — 두 번째 반환값은 사람이 읽을 경고
  - `const DefaultURL = "…"`, `const ConfigTemplate = "…"`

- [ ] **Step 1: 실패하는 테스트 작성**

`internal/holiday/store_test.go`:

```go
package holiday

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newPaths 는 임시 디렉토리에 설정과 저장 파일 위치를 잡는다. body 가 비면 설정 파일을 만들지 않는다.
func newPaths(t *testing.T, body string) Paths {
	t.Helper()
	dir := t.TempDir()
	p := Paths{Config: filepath.Join(dir, "holiday.toml"), StateDir: filepath.Join(dir, "state")}
	if body != "" {
		if err := os.WriteFile(p.Config, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

// storeSample 은 테스트에서 저장 파일로 쓰는 목록이다.
var storeSample = File{
	SyncedAt: "2026-09-19T10:00:00+09:00",
	Source:   DefaultURL,
	Covers:   Range{From: "2026-01-01", To: "2026-12-31"},
	Holidays: []Entry{
		{Date: "2026-03-01", Name: "삼일절"},
		{Date: "2026-03-02", Name: "쉬는 날 삼일절"},
		{Date: "2026-05-01", Name: "노동절"},
	},
}

func TestLoadConfigMissingFileUsesDefaults(t *testing.T) {
	cfg, err := LoadConfig(newPaths(t, "").Config)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.URL != DefaultURL || len(cfg.Extra) != 0 || len(cfg.Ignore) != 0 {
		t.Errorf("missing config should fall back to defaults, got %+v", cfg)
	}
}

func TestLoadConfigRejectsBadDates(t *testing.T) {
	_, err := LoadConfig(newPaths(t, "extra = [\"2026-13-99\"]\nignore = [\"nope\"]\n").Config)
	if err == nil {
		t.Fatal("bad dates should be an error")
	}
	for _, want := range []string{"2026-13-99", "nope"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q, got: %v", want, err)
		}
	}
}

func TestLoadConfigRejectsUnknownKey(t *testing.T) {
	if _, err := LoadConfig(newPaths(t, "urls = \"x\"\n").Config); err == nil {
		t.Fatal("unknown key should be an error")
	}
}

func TestWriteThenReadFile(t *testing.T) {
	p := newPaths(t, "")
	if err := WriteFile(p.Calendar(), storeSample); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := ReadFile(p.Calendar())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(got.Holidays) != 3 || got.Holidays[0].Name != "삼일절" || got.Covers.To != "2026-12-31" {
		t.Errorf("round trip lost data: %+v", got)
	}
}

func TestOpenAppliesConfig(t *testing.T) {
	p := newPaths(t, "extra = [\"2026-10-12\"]\nignore = [\"2026-05-01\"]\n")
	if err := WriteFile(p.Calendar(), storeSample); err != nil {
		t.Fatal(err)
	}
	cal, warning, err := Open(p, day(t, "2026-03-03"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if warning != "" {
		t.Errorf("a fresh list should not warn, got %q", warning)
	}
	if reason, off := cal.Holiday(day(t, "2026-03-02")); !off || reason != "쉬는 날 삼일절" {
		t.Errorf("stored holiday: got (%q, %v)", reason, off)
	}
	if !cal.IsWorkday(day(t, "2026-05-01")) {
		t.Error("ignore should remove 2026-05-01")
	}
	if cal.IsWorkday(day(t, "2026-10-12")) {
		t.Error("extra should add 2026-10-12")
	}
}

func TestOpenWithoutStoredList(t *testing.T) {
	p := newPaths(t, "")
	cal, warning, err := Open(p, day(t, "2026-03-02"))
	if err != nil {
		t.Fatalf("Open should not fail without a stored list: %v", err)
	}
	if !strings.Contains(warning, "gofer holiday sync") {
		t.Errorf("warning should point at sync, got %q", warning)
	}
	if !cal.IsWorkday(day(t, "2026-03-02")) {
		t.Error("a public holiday cannot be detected without a stored list")
	}
	if cal.IsWorkday(day(t, "2026-03-14")) {
		t.Error("saturday is still a day off")
	}
}

func TestOpenOutsideCoveredRange(t *testing.T) {
	p := newPaths(t, "")
	if err := WriteFile(p.Calendar(), storeSample); err != nil {
		t.Fatal(err)
	}
	_, warning, err := Open(p, day(t, "2027-03-02"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !strings.Contains(warning, "gofer holiday sync") {
		t.Errorf("a stale list should warn, got %q", warning)
	}
}

func TestOpenFailsOnBrokenConfig(t *testing.T) {
	p := newPaths(t, "extra = [\"nope\"]\n")
	if _, _, err := Open(p, day(t, "2026-03-03")); err == nil {
		t.Fatal("a broken config must be an error — a person has to fix it")
	}
}

func TestOpenSurvivesBrokenStoredList(t *testing.T) {
	p := newPaths(t, "")
	if err := os.MkdirAll(p.StateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.Calendar(), []byte("{ broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	cal, warning, err := Open(p, day(t, "2026-03-02"))
	if err != nil {
		t.Fatalf("a broken stored list is a warning, not an error: %v", err)
	}
	if !strings.Contains(warning, "gofer holiday sync") {
		t.Errorf("warning should point at sync, got %q", warning)
	}
	if cal.IsWorkday(day(t, "2026-03-14")) {
		t.Error("saturday is still a day off")
	}
}
```

- [ ] **Step 2: 테스트가 실패하는지 확인**

Run: `go test ./internal/holiday/ -run 'TestLoadConfig|TestWriteThenRead|TestOpen' -v`
Expected: 빌드 실패 — `undefined: Paths`, `undefined: LoadConfig`, `undefined: Open`

- [ ] **Step 3: 최소 구현 작성**

`internal/holiday/paths.go`:

```go
package holiday

import (
	"os"
	"path/filepath"
)

// Paths 는 holiday 도구가 쓰는 파일 위치다. 테스트는 임시 디렉토리로 직접 만든다.
type Paths struct {
	Config   string // ~/.config/gofer/holiday.toml — 사람만 쓴다
	StateDir string // ~/Library/Application Support/gofer/holiday — sync 만 쓴다
}

// Calendar 는 내려받은 목록을 저장하는 파일이다.
func (p Paths) Calendar() string { return filepath.Join(p.StateDir, "calendar.json") }

// DefaultPaths 는 홈 디렉토리 기준 기본 위치를 만든다.
func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	return Paths{
		Config:   filepath.Join(home, ".config", "gofer", "holiday.toml"),
		StateDir: filepath.Join(home, "Library", "Application Support", "gofer", "holiday"),
	}, nil
}
```

`internal/holiday/store.go`:

```go
package holiday

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

// DefaultURL 은 대한민국 공휴일 달력이다. 설정에서 주소를 바꾸면 다른 나라 달력도 쓸 수 있다.
const DefaultURL = "https://calendar.google.com/calendar/ical/ko.south_korea%23holiday%40group.v.calendar.google.com/public/basic.ics"

// Config 는 ~/.config/gofer/holiday.toml 이다. 사람만 쓰고 프로그램은 읽기만 한다 —
// 그래서 sync 를 몇 번 돌려도 손으로 적어 둔 내용과 주석이 날아가지 않는다.
type Config struct {
	URL    string   `toml:"url"`
	Extra  []string `toml:"extra"`
	Ignore []string `toml:"ignore"`
}

// ConfigTemplate 은 `holiday config init` 이 쓰는 설정 템플릿이다.
const ConfigTemplate = `# 내려받을 달력 주소. 비우면 대한민국 공휴일 달력을 쓴다.
url = "` + DefaultURL + `"

# 달력에 없지만 쉬는 날로 칠 날짜. 임시공휴일이 갑자기 지정됐을 때 여기에 적는다.
extra = []

# 달력에 있지만 쉬는 날로 치지 않을 날짜.
# 이 달력은 노동절(5/1)과 제헌절(7/17)을 공휴일로 싣는데, 연도마다 기준이 다르고
# 관공서 기준과도 어긋난다. 그날 일한다면 여기에 적는다.
ignore = []
`

// LoadConfig 는 설정을 읽는다. 파일이 없으면 기본값을 돌려준다 (오류가 아니다).
// 날짜 형식 오류와 모르는 항목은 전부 모아 하나의 오류로 돌려준다.
func LoadConfig(path string) (Config, error) {
	cfg := Config{URL: DefaultURL}
	md, err := toml.DecodeFile(path, &cfg)
	if errors.Is(err, os.ErrNotExist) {
		return Config{URL: DefaultURL}, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("read %s: %w", path, err)
	}
	var errs []error
	for _, k := range md.Undecoded() {
		errs = append(errs, fmt.Errorf("unknown key %q", k.String()))
	}
	if cfg.URL == "" {
		cfg.URL = DefaultURL
	}
	errs = append(errs, checkDates("extra", cfg.Extra)...)
	errs = append(errs, checkDates("ignore", cfg.Ignore)...)
	if len(errs) > 0 {
		return cfg, fmt.Errorf("config %s:\n%w", path, errors.Join(errs...))
	}
	return cfg, nil
}

func checkDates(field string, dates []string) []error {
	var errs []error
	for _, d := range dates {
		if _, err := time.Parse(dateLayout, d); err != nil {
			errs = append(errs, fmt.Errorf("%s: %q is not YYYY-MM-DD", field, d))
		}
	}
	return errs
}

// Range 는 공휴일 목록이 담고 있는 기간이다.
type Range struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// File 은 calendar.json 이다. sync 만 쓴다.
type File struct {
	SyncedAt string  `json:"synced_at"`
	Source   string  `json:"source"`
	Covers   Range   `json:"covers"`
	Holidays []Entry `json:"holidays"`
}

// ReadFile 은 저장된 목록을 읽는다.
func ReadFile(path string) (File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return File{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(f.Holidays) == 0 {
		return File{}, fmt.Errorf("%s has no holidays", path)
	}
	return f, nil
}

// WriteFile 은 목록을 저장한다. 디렉토리가 없으면 만든다.
func WriteFile(path string, f File) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// Open 은 설정과 저장된 목록을 함께 읽어 판정용 달력을 만든다.
//
// 저장된 목록이 없거나 깨졌거나 now 가 수록 기간 밖이면, 주말만 아는 달력과 사람이 읽을
// 경고 문구를 돌려준다 — 판정할 수 없다고 실행을 막으면 그래프가 조용히 며칠씩 늙기 때문이다.
// 오류를 돌려주는 경우는 설정 파일 자체가 잘못 적혀 있을 때뿐이다. 그건 사람이 고쳐야 한다.
func Open(p Paths, now time.Time) (*Calendar, string, error) {
	cfg, err := LoadConfig(p.Config)
	if err != nil {
		return nil, "", err
	}
	f, rerr := ReadFile(p.Calendar())
	if rerr != nil {
		return New(nil, "", "", cfg.Extra, cfg.Ignore), syncHint(rerr), nil
	}
	cal := New(f.Holidays, f.Covers.From, f.Covers.To, cfg.Extra, cfg.Ignore)
	if !cal.Covers(now) {
		return cal, fmt.Sprintf("holiday list only covers %s~%s — run `gofer holiday sync` (weekends still apply)", f.Covers.From, f.Covers.To), nil
	}
	return cal, "", nil
}

func syncHint(err error) string {
	if errors.Is(err, os.ErrNotExist) {
		return "no holiday list yet — run `gofer holiday sync` (weekends still apply)"
	}
	return fmt.Sprintf("could not read the holiday list (%v) — run `gofer holiday sync` (weekends still apply)", err)
}
```

- [ ] **Step 4: 테스트가 통과하는지 확인**

Run: `go test ./internal/holiday/ -v`
Expected: PASS — Task 1의 네 개를 포함해 전부 통과

- [ ] **Step 5: 커밋**

```bash
git add internal/holiday/paths.go internal/holiday/store.go internal/holiday/store_test.go
git commit -m "feat(holiday): config, stored list and Open

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

**Task 2 완료조건:** `go test ./internal/holiday/ -v`가 PASS이고 `go vet ./internal/holiday/`가 조용하다. `Open`이 설정 오류에서만 오류를 내고, 목록이 없거나 깨졌을 때는 경고만 낸다.

---

### Task 3: 달력 내려받기와 해석

스펙 5절에 적힌 세 가지 함정(CRLF 줄바꿈, 접힌 줄, 끝 날짜가 배타적)을 테스트로 먼저 고정한다.

**Files:**
- Create: `internal/holiday/ics.go`
- Test: `internal/holiday/ics_test.go`

**Interfaces:**
- Consumes: Task 1의 `Entry`, `dateLayout`; Task 2의 `Paths`, `LoadConfig`, `File`, `Range`, `WriteFile`
- Produces:
  - `func ParseICS(body []byte) []Entry` — 날짜순 정렬
  - `func Fetch(ctx context.Context, url string) ([]Entry, error)`
  - `func Sync(ctx context.Context, p Paths, now time.Time) (File, error)`
  - `func OpenOrRefresh(ctx context.Context, p Paths, now time.Time) (*Calendar, string, error)`

- [ ] **Step 1: 실패하는 테스트 작성**

`internal/holiday/ics_test.go`:

```go
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
```

- [ ] **Step 2: 테스트가 실패하는지 확인**

Run: `go test ./internal/holiday/ -run 'TestParseICS|TestFetch|TestSync' -v`
Expected: 빌드 실패 — `undefined: ParseICS`, `undefined: Fetch`, `undefined: Sync`

- [ ] **Step 3: 최소 구현 작성**

`internal/holiday/ics.go`:

```go
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
```

- [ ] **Step 4: 테스트가 통과하는지 확인**

Run: `go test ./internal/holiday/ -v`
Expected: PASS — 전부 통과. 특히 `TestOpenOrRefreshSkipsNetworkWhenFresh`가 요청 건수 0을 확인한다

- [ ] **Step 5: 커밋**

```bash
git add internal/holiday/ics.go internal/holiday/ics_test.go
git commit -m "feat(holiday): download, parse, and refresh a stale list once

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

**Task 3 완료조건:** `go test ./internal/holiday/ -v`가 PASS이고 `go vet ./internal/holiday/`가 조용하다. `grep -n require -A 10 go.mod`로 의존성이 늘지 않았음을 확인한다.

---

### Task 4: `gofer holiday` 명령

**Files:**
- Create: `cmd/holiday/holiday.go`, `cmd/holiday/sync.go`, `cmd/holiday/list.go`, `cmd/holiday/check.go`
- Modify: `cmd/root.go` (import 한 줄, `AddCommand` 한 줄)
- Test: `cmd/root_test.go` (테스트 하나 추가)

**Interfaces:**
- Consumes: Task 2·3의 `DefaultPaths`, `Open`, `Sync`, `ReadFile`, `ConfigTemplate`
- Produces: `func Cmd() *cobra.Command` (패키지 `cmd/holiday`)

- [ ] **Step 1: 실패하는 테스트 작성**

`cmd/root_test.go` 끝에 추가:

```go
func TestRootHelpListsHoliday(t *testing.T) {
	root := Root()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "holiday") {
		t.Fatalf("help should list holiday, got:\n%s", out.String())
	}
}
```

- [ ] **Step 2: 테스트가 실패하는지 확인**

Run: `go test ./cmd/ -run TestRootHelpListsHoliday -v`
Expected: FAIL — `help should list holiday`

- [ ] **Step 3: 명령 구현**

`cmd/holiday/holiday.go`:

```go
// Package holiday 는 `gofer holiday` 서브커맨드 트리다. 플래그 파싱만 하고
// 로직은 internal/holiday 에 둔다.
package holiday

import "github.com/spf13/cobra"

// Cmd 는 `holiday` 부모 커맨드를 만든다.
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "holiday",
		Short: "Keep a local holiday calendar and tell workdays from days off",
		Long: `Downloads a public holiday calendar once and stores it locally, so other tools can ask
"is today a workday?" without touching the network. Weekends need no stored list.`,
	}
	cmd.AddCommand(syncCmd(), listCmd(), checkCmd())
	return cmd
}
```

`cmd/holiday/sync.go`:

```go
package holiday

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	hol "gofer/internal/holiday"
)

func syncCmd() *cobra.Command {
	var initConfig bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Download the holiday calendar and store it locally",
		RunE: func(cmd *cobra.Command, _ []string) error {
			paths, err := hol.DefaultPaths()
			if err != nil {
				return err
			}
			if initConfig {
				return writeConfigTemplate(cmd, paths.Config)
			}
			f, err := hol.Sync(cmd.Context(), paths, time.Now())
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s ~ %s · %d holidays stored\n  %s\n",
				f.Covers.From, f.Covers.To, len(f.Holidays), paths.Calendar())
			return nil
		},
	}
	cmd.Flags().BoolVar(&initConfig, "init-config", false, "write the config template instead of downloading")
	return cmd
}

// writeConfigTemplate 은 설정 템플릿을 0600 으로 새로 만든다. 이미 있으면 건드리지 않는다.
func writeConfigTemplate(cmd *cobra.Command, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(hol.ConfigTemplate); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", path)
	return nil
}
```

`cmd/holiday/list.go`:

```go
package holiday

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	hol "gofer/internal/holiday"
)

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list [year]",
		Short: "Show the stored holidays of a year (default: this year)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			year := strconv.Itoa(time.Now().Year())
			if len(args) == 1 {
				if _, err := strconv.Atoi(args[0]); err != nil || len(args[0]) != 4 {
					return fmt.Errorf("year must be four digits, got %q", args[0])
				}
				year = args[0]
			}
			paths, err := hol.DefaultPaths()
			if err != nil {
				return err
			}
			f, err := hol.ReadFile(paths.Calendar())
			if err != nil {
				return fmt.Errorf("%w — run `gofer holiday sync` first", err)
			}
			n := 0
			for _, e := range f.Holidays {
				if strings.HasPrefix(e.Date, year) {
					fmt.Fprintf(cmd.OutOrStdout(), "%s  %s\n", e.Date, e.Name)
					n++
				}
			}
			if n == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no holidays stored for %s (list covers %s ~ %s)\n", year, f.Covers.From, f.Covers.To)
			}
			return nil
		},
	}
}
```

`cmd/holiday/check.go`:

```go
package holiday

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	hol "gofer/internal/holiday"
)

func checkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check [YYYY-MM-DD]",
		Short: "Tell whether a date is a workday (default: today)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			when := time.Now()
			if len(args) == 1 {
				parsed, err := time.ParseInLocation("2006-01-02", args[0], time.Local)
				if err != nil {
					return fmt.Errorf("date must be YYYY-MM-DD, got %q", args[0])
				}
				when = parsed
			}
			paths, err := hol.DefaultPaths()
			if err != nil {
				return err
			}
			cal, warning, err := hol.Open(paths, when)
			if err != nil {
				return err
			}
			if warning != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: "+warning)
			}
			if reason, off := cal.Holiday(when); off {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  holiday — %s\n", when.Format("2006-01-02 (Mon)"), reason)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s  workday\n", when.Format("2006-01-02 (Mon)"))
			return nil
		},
	}
}
```

`cmd/root.go`를 고친다. import에 `"gofer/cmd/holiday"`를 더하고, `root.AddCommand(uarefresh.Cmd())` 다음 줄에 다음을 넣는다.

```go
	root.AddCommand(holiday.Cmd())
```

- [ ] **Step 4: 테스트가 통과하는지 확인**

Run: `go test ./cmd/ -v && go build -o bin/gofer . && ./bin/gofer holiday --help`
Expected: PASS, 그리고 `sync`·`list`·`check` 세 개가 도움말에 보인다

- [ ] **Step 5: 손으로 한 번 확인**

Run: `./bin/gofer holiday sync && ./bin/gofer holiday list 2026 | head -5 && ./bin/gofer holiday check 2026-03-02 && ./bin/gofer holiday check 2026-03-03`
Expected:
```
2021-01-01 ~ 2031-12-31 · 205 holidays stored
  …/gofer/holiday/calendar.json
2026-01-01  새해첫날
…
2026-03-02 (Mon)  holiday — 쉬는 날 삼일절
2026-03-03 (Tue)  workday
```
(건수는 달력이 바뀌면 달라질 수 있다. 200건 안팎이면 정상이다.)

- [ ] **Step 6: 커밋**

```bash
git add cmd/holiday/ cmd/root.go cmd/root_test.go
git commit -m "feat(holiday): sync, list and check commands

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

**Task 4 완료조건:** `go test ./...`가 PASS이고, Step 5의 네 명령이 위와 같은 모양으로 출력한다.

---

### Task 5: `ua-refresh` 설정에 `workdays_only`

**여기에 이 플랜에서 가장 틀리기 쉬운 지점이 있다.** Go에서 `bool` 항목은 값이 없으면 자동으로 `false`가 되므로, 그냥 읽으면 "항목 없음"과 "`false`라고 적음"이 구별되지 않아 **기본값이 켜짐에서 꺼짐으로 조용히 뒤집힌다.** TOML 해석 결과의 `IsDefined`로 항목이 실제로 적혀 있었는지 확인해야 한다.

**Files:**
- Modify: `internal/uarefresh/config.go` (`ScheduleConfig`, `Load`, `Template`)
- Test: `internal/uarefresh/config_test.go` (테스트 세 개 추가)

**Interfaces:**
- Consumes: 없음
- Produces: `ScheduleConfig.WorkdaysOnly bool` (`toml:"workdays_only"`)

- [ ] **Step 1: 실패하는 테스트 작성**

`internal/uarefresh/config_test.go` 끝에 추가:

```go
// TestWorkdaysOnlyDefaultsToOn 은 항목을 적지 않은 기존 설정 파일이 켜진 것으로 읽히는지 본다.
// Go 의 bool 기본값은 false 라서, 그냥 읽으면 여기서 조용히 꺼짐으로 뒤집힌다.
func TestWorkdaysOnlyDefaultsToOn(t *testing.T) {
	cfg, err := Load(writeConfig(t, validConfig))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Schedule.WorkdaysOnly {
		t.Error("workdays_only should default to true when the key is absent")
	}
}

func TestWorkdaysOnlyCanBeTurnedOff(t *testing.T) {
	body := strings.Replace(validConfig, `at = "07:30"`, "at = \"07:30\"\nworkdays_only = false", 1)
	cfg, err := Load(writeConfig(t, body))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Schedule.WorkdaysOnly {
		t.Error("workdays_only = false must be honoured")
	}
}

func TestWorkdaysOnlyExplicitTrue(t *testing.T) {
	body := strings.Replace(validConfig, `at = "07:30"`, "at = \"07:30\"\nworkdays_only = true", 1)
	cfg, err := Load(writeConfig(t, body))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Schedule.WorkdaysOnly {
		t.Error("workdays_only = true must be honoured")
	}
}
```

- [ ] **Step 2: 테스트가 실패하는지 확인**

Run: `go test ./internal/uarefresh/ -run TestWorkdaysOnly -v`
Expected: 빌드 실패 — `cfg.Schedule.WorkdaysOnly undefined`

- [ ] **Step 3: 최소 구현 작성**

`internal/uarefresh/config.go`의 `ScheduleConfig`를 다음으로 바꾼다.

```go
type ScheduleConfig struct {
	At           string `toml:"at"`            // "HH:MM"
	WorkdaysOnly bool   `toml:"workdays_only"` // 적지 않으면 true (Load 참고)
}
```

`Load` 안에서 `md.Undecoded()` 루프 바로 다음에 넣는다.

```go
	// workdays_only 는 적지 않으면 켜진 것으로 본다. Go 의 bool 기본값이 false 라서
	// 그냥 두면 "항목 없음" 과 "false 라고 적음" 이 구별되지 않고 기본값이 뒤집힌다.
	if !md.IsDefined("schedule", "workdays_only") {
		cfg.Schedule.WorkdaysOnly = true
	}
```

`Template`의 `[schedule]` 절을 다음으로 바꾼다.

```go
const Template = `[schedule]
at            = "07:30"      # install 이 plist 에 반영
workdays_only = true         # 토·일·공휴일에는 실행하지 않는다 (먼저 `+"`gofer holiday sync`"+` 로 목록을 받는다)
```

(`Template`은 백틱 문자열이므로, 안에 백틱을 넣으려면 위처럼 문자열을 이어 붙인다.)

- [ ] **Step 4: 테스트가 통과하는지 확인**

Run: `go test ./internal/uarefresh/ -run 'TestWorkdaysOnly|TestLoad' -v`
Expected: PASS — 세 개의 새 테스트와 기존 설정 테스트가 모두 통과

- [ ] **Step 5: 커밋**

```bash
git add internal/uarefresh/config.go internal/uarefresh/config_test.go
git commit -m "feat(ua-refresh): workdays_only config key, on unless set to false

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

**Task 5 완료조건:** `go test ./internal/uarefresh/ -run TestWorkdaysOnly -v`가 PASS. 특히 항목이 없는 `validConfig`가 `true`로 읽힌다.

---

### Task 6: 쉬는 날 게이트와 `--force`

**Files:**
- Modify: `internal/uarefresh/command.go` (`CommandOptions`에 두 필드, `RunCommand`에 게이트, 새 함수 `holidayGate`)
- Modify: `internal/uarefresh/result.go` (`RunResult`에 `Warning` 필드)
- Modify: `internal/uarefresh/report.go` (`SlackText` 끝에 경고 한 줄)
- Modify: `cmd/uarefresh/run.go` (`--force` 플래그와 달력 경로 전달)
- Test: `internal/uarefresh/command_test.go` (테스트 네 개 추가)

**Interfaces:**
- Consumes: Task 2의 `holiday.Paths`·`holiday.Open`, Task 3의 `holiday.OpenOrRefresh`, Task 5의 `cfg.Schedule.WorkdaysOnly`
- Produces:
  - `CommandOptions.Force bool`, `CommandOptions.Holiday holiday.Paths`
  - `RunResult.Warning string` (`json:"warning,omitempty"`)
  - `func holidayGate(ctx context.Context, o CommandOptions, cfg *Config, now time.Time) (reason, warning string, err error)`

- [ ] **Step 1: 실패하는 테스트 작성**

`internal/uarefresh/command_test.go` 끝에 추가:

```go
// TestRunCommandSkipsOnWeekend 는 토요일에 아무것도 하지 않고 끝나는지 본다. Slack 을 보내지
// 않는 것이 이 기능의 목적이고, 잠금·로그 파일·last-run.json 도 건드리지 않아야 한다.
// setupCommand 가 만드는 설정에는 workdays_only 가 없다 — 없어도 켜진 것으로 읽혀야 한다.
func TestRunCommandSkipsOnWeekend(t *testing.T) {
	work, _ := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "ok")
	slack := newSlackStub(t, 200)
	o, out := setupCommand(t, f, slack.srv.URL, RepoConfig{Path: work, Trunk: "main"})
	o.Now = func() time.Time { return time.Date(2026, 9, 12, 7, 30, 0, 0, time.Local) } // 토요일

	if err := RunCommand(t.Context(), o); err != nil {
		t.Fatalf("a skipped day must exit cleanly: %v", err)
	}
	if slack.count() != 0 {
		t.Errorf("no Slack message should go out, got %d", slack.count())
	}
	if !strings.Contains(out.String(), "skipping") || !strings.Contains(out.String(), "주말") {
		t.Errorf("stdout should say why it skipped:\n%s", out.String())
	}
	if _, err := os.Stat(o.Paths.LastRun()); !errors.Is(err, os.ErrNotExist) {
		t.Error("last-run.json must not be touched on a day off")
	}
	if _, err := os.Stat(o.Paths.DailyLog(o.Now())); !errors.Is(err, os.ErrNotExist) {
		t.Error("no daily log file should be created on a day off")
	}
	if _, err := os.Stat(o.Paths.Lock()); !errors.Is(err, os.ErrNotExist) {
		t.Error("no lock file should be left behind")
	}
}

func TestRunCommandRunsOnWorkday(t *testing.T) {
	work, origin := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "ok")
	pushFromClone(t, origin, "main", "a.txt", "a")
	slack := newSlackStub(t, 200)
	o, _ := setupCommand(t, f, slack.srv.URL, RepoConfig{Path: work, Trunk: "main"})
	o.Now = func() time.Time { return time.Date(2026, 9, 11, 7, 30, 0, 0, time.Local) } // 금요일

	if err := RunCommand(t.Context(), o); err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	if slack.count() != 2 {
		t.Errorf("a workday sends start and result messages, got %d", slack.count())
	}
}

// TestRunCommandForceOverridesHoliday 는 --force 가 토요일 판정을 무시하는지 본다.
func TestRunCommandForceOverridesHoliday(t *testing.T) {
	work, origin := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "ok")
	pushFromClone(t, origin, "main", "a.txt", "a")
	slack := newSlackStub(t, 200)
	o, _ := setupCommand(t, f, slack.srv.URL, RepoConfig{Path: work, Trunk: "main"})
	o.Now = func() time.Time { return time.Date(2026, 9, 12, 7, 30, 0, 0, time.Local) } // 토요일
	o.Force = true

	if err := RunCommand(t.Context(), o); err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	if slack.count() != 2 {
		t.Errorf("--force should run normally, got %d messages", slack.count())
	}
}

// TestRunCommandPutsHolidayWarningInSlack 은 목록을 회복하지 못했을 때 그 사실이 결과
// 알림에 실리는지 본다. 표준 출력으로만 내보내면 launchd.log 에 쌓이고 아무도 보지 않는다.
// 설정의 url 이 닿지 않는 주소이므로 자동 회복도 실패한다.
func TestRunCommandPutsHolidayWarningInSlack(t *testing.T) {
	work, origin := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "ok")
	pushFromClone(t, origin, "main", "a.txt", "a")
	slack := newSlackStub(t, 200)
	o, _ := setupCommand(t, f, slack.srv.URL, RepoConfig{Path: work, Trunk: "main"})
	o.Now = func() time.Time { return time.Date(2026, 9, 11, 7, 30, 0, 0, time.Local) } // 금요일

	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer dead.Close()
	if err := os.MkdirAll(filepath.Dir(o.Holiday.Config), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(o.Holiday.Config, []byte("url = \""+dead.URL+"\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := RunCommand(t.Context(), o); err != nil {
		t.Fatalf("a failed recovery must not block the run: %v", err)
	}
	if !strings.Contains(slack.last(), "gofer holiday sync") {
		t.Errorf("the result message should carry the warning, got:\n%s", slack.last())
	}
	last, err := ReadRunResult(o.Paths.LastRun())
	if err != nil || !strings.Contains(last.Warning, "gofer holiday sync") {
		t.Errorf("last-run.json should keep the warning: %v %+v", err, last.Warning)
	}
}
```

`setupCommand`가 `CommandOptions`에 `Holiday`를 채우도록 같은 파일의 헬퍼를 고친다. `paths` 블록 다음에 한 줄을 더하고, 마지막 반환문을 바꾼다.

```go
	hpaths := holiday.Paths{
		Config:   filepath.Join(base, "config", "holiday.toml"),
		StateDir: filepath.Join(base, "state", "holiday"),
	}
	…
	return CommandOptions{Paths: paths, Holiday: hpaths, Stdout: &out, TTY: false}, &out
```

`command_test.go`의 import에 `"gofer/internal/holiday"`를 더한다.

- [ ] **Step 2: 테스트가 실패하는지 확인**

Run: `go test ./internal/uarefresh/ -run 'TestRunCommandSkips|TestRunCommandRunsOn|TestRunCommandForce' -v`
Expected: 빌드 실패 — `unknown field Holiday`, `unknown field Force`

- [ ] **Step 3: 최소 구현 작성**

`internal/uarefresh/command.go`의 import에 `"gofer/internal/holiday"`를 더하고, `CommandOptions`에 두 필드를 넣는다.

```go
type CommandOptions struct {
	Paths   Paths
	Holiday holiday.Paths
	DryRun  bool
	Force   bool
	Only    string
	Stdout  io.Writer
	TTY     bool
	Now     func() time.Time
}
```

같은 파일에 함수를 더한다.

```go
// holidayGate 는 오늘이 쉬는 날인지 본다. 건너뛸 사유와 사람이 볼 경고를 돌려주고,
// 일하는 날이면 사유가 빈 문자열이다. workdays_only 가 꺼져 있거나 --force 면 판정하지 않는다.
//
// 저장된 목록이 오늘을 덮지 못하면 그때만 한 번 내려받아 회복한다 — 사람이 sync 를 잊어도
// 공휴일 판정이 조용히 멎지 않게 하기 위해서다. 회복이 실패해도 오류를 내지 않는다.
// 판정할 수 없다고 실행을 막으면 그래프가 조용히 늙기 때문이다.
// --dry-run 은 아무것도 바꾸지 않아야 하므로 회복을 시도하지 않는다.
func holidayGate(ctx context.Context, o CommandOptions, cfg *Config, now time.Time) (reason, warning string, err error) {
	if !cfg.Schedule.WorkdaysOnly || o.Force {
		return "", "", nil
	}
	var cal *holiday.Calendar
	if o.DryRun {
		cal, warning, err = holiday.Open(o.Holiday, now)
	} else {
		cal, warning, err = holiday.OpenOrRefresh(ctx, o.Holiday, now)
	}
	if err != nil {
		return "", "", err
	}
	reason, _ = cal.Holiday(now)
	return reason, warning, nil
}
```

`RunCommand` 안에서 `selectRepos` 다음, `if o.DryRun` 앞에 게이트를 넣는다.

```go
	reason, holidayWarning, err := holidayGate(ctx, o, cfg, now())
	if err != nil {
		return err
	}
	if holidayWarning != "" {
		fmt.Fprintln(o.Stdout, "warning: "+holidayWarning)
	}
	if o.DryRun {
		if reason != "" {
			fmt.Fprintf(o.Stdout, "%s is a day off (%s) — a real run would skip today\n", now().Format("2006-01-02 (Mon)"), reason)
		}
		return dryRun(ctx, o.Stdout, cfg, repos)
	}
	if reason != "" {
		fmt.Fprintf(o.Stdout, "%s is a day off (%s) — skipping. Use --force to run anyway.\n", now().Format("2006-01-02 (Mon)"), reason)
		return nil
	}
```

기존의 `if o.DryRun { return dryRun(...) }` 블록은 위 코드가 대신하므로 지운다.

같은 함수에서 결과를 받은 뒤, `res.LogPath = logPath` 다음 줄에 경고를 싣는다. 이래야 알림과 `last-run.json` 양쪽에 남는다.

```go
	res.Warning = holidayWarning
```

`internal/uarefresh/result.go`의 `RunResult`에 필드를 더한다.

```go
type RunResult struct {
	StartedAt  time.Time    `json:"started_at"`
	FinishedAt time.Time    `json:"finished_at"`
	Repos      []RepoResult `json:"repos"`
	LogPath    string       `json:"log_path"`
	Warning    string       `json:"warning,omitempty"` // 공휴일 목록을 회복하지 못했을 때만 채워진다
}
```

`internal/uarefresh/report.go`의 `SlackText` 마지막 `fmt.Fprintf(&b, "로그: %s", shortenHome(r.LogPath))` 다음에 세 줄을 더한다.

```go
	if r.Warning != "" {
		fmt.Fprintf(&b, "\n⚠️ %s", r.Warning)
	}
```

`cmd/uarefresh/run.go`를 고친다. import에 `hol "gofer/internal/holiday"`를 더하고, `var dryRun bool` 옆에 `var force bool`을 둔다. `RunE` 안을 다음으로 바꾼다.

```go
			paths, err := ua.DefaultPaths()
			if err != nil {
				return err
			}
			hpaths, err := hol.DefaultPaths()
			if err != nil {
				return err
			}
			return ua.RunCommand(cmd.Context(), ua.CommandOptions{
				Paths: paths, Holiday: hpaths, DryRun: dryRun, Force: force, Only: only,
				Stdout: os.Stdout, TTY: tui.IsTerminal(os.Stdout),
			})
```

플래그 등록 줄 옆에 다음을 더한다.

```go
	cmd.Flags().BoolVar(&force, "force", false, "run even on a weekend or public holiday")
```

- [ ] **Step 4: 테스트가 통과하는지 확인**

Run: `go test ./internal/uarefresh/ -v 2>&1 | tail -20 && go build -o bin/gofer .`
Expected: PASS — 새 테스트 세 개를 포함해 전부 통과

- [ ] **Step 5: 커밋**

```bash
git add internal/uarefresh/command.go internal/uarefresh/command_test.go \
        internal/uarefresh/result.go internal/uarefresh/report.go cmd/uarefresh/run.go
git commit -m "feat(ua-refresh): skip weekends and holidays, --force to override

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

**Task 6 완료조건:** `go test ./internal/uarefresh/ -v`가 PASS. 특히 (1) 토요일 테스트에서 Slack 건수가 0이고 `last-run.json`·일별 로그·잠금 파일이 모두 만들어지지 않으며, (2) 회복 실패 테스트에서 경고가 Slack 본문과 `last-run.json`에 모두 들어간다.

---

### Task 7: `status`에 오늘 판정 표시

**Files:**
- Modify: `internal/uarefresh/status.go` (`StatusText` 시그니처와 첫 두 줄)
- Modify: `cmd/uarefresh/status.go` (달력 경로와 현재 시각 전달)
- Test: `internal/uarefresh/status_test.go` (테스트 두 개 추가)

**Interfaces:**
- Consumes: Task 2의 `holiday.Paths`·`holiday.Open`, Task 5의 `cfg.Schedule.WorkdaysOnly`
- Produces: `func StatusText(ctx context.Context, paths Paths, hpaths holiday.Paths, cfg *Config, installed bool, now time.Time) string`

- [ ] **Step 1: 실패하는 테스트 작성**

`internal/uarefresh/status_test.go` 끝에 추가:

```go
// TestStatusTextShowsTodayOff 는 쉬는 날에 status 가 그 사실을 알려 주는지 본다.
func TestStatusTextShowsTodayOff(t *testing.T) {
	cfg := &Config{Schedule: ScheduleConfig{At: "07:30", WorkdaysOnly: true}}
	base := t.TempDir()
	hpaths := holiday.Paths{
		Config:   filepath.Join(base, "holiday.toml"),
		StateDir: filepath.Join(base, "holiday"),
	}
	saturday := time.Date(2026, 9, 12, 7, 30, 0, 0, time.Local)
	got := StatusText(t.Context(), Paths{StateDir: base}, hpaths, cfg, true, saturday)

	if !strings.Contains(got, "workdays only") {
		t.Errorf("status should say the schedule is workdays only:\n%s", got)
	}
	if !strings.Contains(got, "주말") || !strings.Contains(got, "not running today") {
		t.Errorf("status should show today's judgement:\n%s", got)
	}
}

// TestStatusTextOmitsTodayWhenOff 는 기능을 껐을 때 오늘 줄이 나오지 않는지 본다.
func TestStatusTextOmitsTodayWhenOff(t *testing.T) {
	cfg := &Config{Schedule: ScheduleConfig{At: "07:30", WorkdaysOnly: false}}
	base := t.TempDir()
	hpaths := holiday.Paths{Config: filepath.Join(base, "holiday.toml"), StateDir: filepath.Join(base, "holiday")}
	saturday := time.Date(2026, 9, 12, 7, 30, 0, 0, time.Local)
	got := StatusText(t.Context(), Paths{StateDir: base}, hpaths, cfg, true, saturday)

	if strings.Contains(got, "workdays only") || strings.Contains(got, "not running today") {
		t.Errorf("with workdays_only off, status should not mention it:\n%s", got)
	}
}
```

`status_test.go`의 import에 `"path/filepath"`, `"time"`, `"gofer/internal/holiday"`가 없으면 더한다.

- [ ] **Step 2: 테스트가 실패하는지 확인**

Run: `go test ./internal/uarefresh/ -run TestStatusText -v`
Expected: 빌드 실패 — `too many arguments in call to StatusText`

- [ ] **Step 3: 최소 구현 작성**

`internal/uarefresh/status.go`의 import에 `"time"`과 `"gofer/internal/holiday"`를 더하고, `StatusText` 앞부분을 다음으로 바꾼다.

```go
// StatusText 는 `gofer ua-refresh status` 본문이다: launchd 등록 여부 · 오늘 판정 ·
// 마지막 실행 · 레포별 그래프 신선도.
func StatusText(ctx context.Context, paths Paths, hpaths holiday.Paths, cfg *Config, installed bool, now time.Time) string {
	var b strings.Builder
	only := ""
	if cfg.Schedule.WorkdaysOnly {
		only = " · workdays only"
	}
	if installed {
		fmt.Fprintf(&b, "launchd: installed · daily at %s%s · %s\n", cfg.Schedule.At, only, shortenHome(paths.Plist))
	} else {
		b.WriteString("launchd: not installed (run `gofer ua-refresh install`)\n")
	}
	if cfg.Schedule.WorkdaysOnly {
		b.WriteString(todayLine(hpaths, now))
	}
```

같은 파일에 함수를 더한다.

```go
// todayLine 은 오늘이 쉬는 날인지 한 줄로 알려 준다. 목록이 없거나 낡았으면 그 사실도 붙인다.
func todayLine(hpaths holiday.Paths, now time.Time) string {
	cal, warning, err := holiday.Open(hpaths, now)
	if err != nil {
		return fmt.Sprintf("today:   %s · holiday config error: %v\n", now.Format("2006-01-02 (Mon)"), err)
	}
	line := fmt.Sprintf("today:   %s · workday\n", now.Format("2006-01-02 (Mon)"))
	if reason, off := cal.Holiday(now); off {
		line = fmt.Sprintf("today:   %s · day off (%s) · not running today\n", now.Format("2006-01-02 (Mon)"), reason)
	}
	if warning != "" {
		line += "         " + warning + "\n"
	}
	return line
}
```

`cmd/uarefresh/status.go`의 import에 `hol "gofer/internal/holiday"`와 `"time"`을 더하고, `StatusText` 호출을 바꾼다.

```go
			hpaths, err := hol.DefaultPaths()
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), ua.StatusText(cmd.Context(), paths, hpaths, cfg, installed, time.Now()))
```

- [ ] **Step 4: 테스트가 통과하는지 확인**

Run: `go test ./... 2>&1 | tail -10 && go build -o bin/gofer . && ./bin/gofer ua-refresh status`
Expected: 전부 PASS. `status` 출력에 `workdays only`와 `today:` 줄이 보인다

- [ ] **Step 5: 커밋**

```bash
git add internal/uarefresh/status.go internal/uarefresh/status_test.go cmd/uarefresh/status.go
git commit -m "feat(ua-refresh): show today's workday judgement in status

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

**Task 7 완료조건:** `go test ./...`가 PASS이고, `./bin/gofer ua-refresh status`가 `today:` 줄을 출력한다.

---

### Task 8: 문서 갱신과 최종 검증

**Files:**
- Modify: `README.md` (Usage·Configuration 절에 `holiday` 추가)
- Modify: `docs/superpowers/specs/2026-09-12-holiday-workdays-design.md` (6절 화면 예시를 실제 영어 출력에 맞춤)
- Modify: `CLAUDE.md` (도구가 둘이 되었으므로 한 줄)

- [ ] **Step 1: README의 Usage 절에 새 도구를 더한다**

`gofer ua-refresh log` 줄 아래에 다음을 넣는다.

```
gofer holiday sync                                             공휴일 달력을 내려받아 로컬에 저장 (연 1회 정도)
gofer holiday list [연도]                                       저장된 공휴일 목록 출력
gofer holiday check [YYYY-MM-DD]                                그날이 쉬는 날인지 확인 (생략 시 오늘)
```

- [ ] **Step 2: README의 Configuration 절에 설명을 더한다**

`ua-refresh.toml` 설명 다음에 아래 문단을 넣는다.

```markdown
### 쉬는 날에는 실행하지 않기

`ua-refresh`는 기본적으로 토요일·일요일·공휴일·대체공휴일에 실행되지 않고, 그런 날에는 Slack 알림도 보내지 않는다. 주말은 별도 준비 없이 판정되지만 공휴일은 목록이 있어야 하므로, 설치 후 `gofer holiday sync`를 한 번 실행해 달력을 내려받아 둔다.

목록이 오늘을 덮지 못하면 `ua-refresh`가 그때 한 번 스스로 내려받아 회복하므로, `sync`를 주기적으로 돌릴 필요는 없다(내려받은 달력은 여러 해치를 한꺼번에 담는다). 회복까지 실패한 경우에만 주말만 걸러 내고, 그 사실을 그날 결과 알림 맨 아래에 한 줄로 알린다.

쉬는 날에도 실행하려면 `~/.config/gofer/ua-refresh.toml`의 `[schedule]`에서 `workdays_only = false`로 두거나, 한 번만 무시할 때는 `gofer ua-refresh run --force`를 쓴다.

공휴일 판정을 손보려면 `~/.config/gofer/holiday.toml`을 만든다(`gofer holiday sync --init-config`가 템플릿을 만들어 준다). `extra`에 적은 날짜는 쉬는 날로 더해지고, `ignore`에 적은 날짜는 쉬는 날에서 빠진다. 이 파일은 `sync`가 건드리지 않으므로 직접 적은 내용이 사라지지 않는다.
```

- [ ] **Step 3: 설계 문서 6절의 화면 예시를 실제 출력에 맞춘다**

`docs/superpowers/specs/2026-09-12-holiday-workdays-design.md`의 "## 6. 화면과 로그" 블록 안의 한국어 예시를 Task 4·6·7에서 실제로 구현한 영어 문구로 바꾼다. 휴일 이름은 달력에서 온 한국어 그대로 둔다. 바꾼 뒤 그 절 첫 줄에 한 문장을 더한다.

```markdown
사용자에게 보이는 문구는 기존 `ua-refresh` 출력과 같이 영어로 쓰고, 달력에서 읽어 온 휴일 이름만 한국어 그대로 둔다.
```

- [ ] **Step 4: `CLAUDE.md`에 도구가 둘이 되었음을 반영한다**

`## 기술 결정` 절의 "새 도구 = ..." 줄은 그대로 두고, 파일 맨 위 소개 문장 다음에 한 줄을 더한다.

```markdown
현재 도구는 둘이다: `ua-refresh`(지식 그래프 갱신)와 `holiday`(공휴일 달력 저장·판정, 다른 도구가 코드로도 부른다).
```

- [ ] **Step 5: 전체 검증을 실제로 실행한다**

```bash
cd /Users/jaeyoungcho/lab/gofer
go vet ./... && go test ./...
go build -o bin/gofer . && ./bin/gofer --help
./bin/gofer holiday sync
./bin/gofer holiday check 2026-03-02
./bin/gofer holiday check 2026-03-03
./bin/gofer ua-refresh status
./bin/gofer ua-refresh run --dry-run
git diff 4a7f9f2 --stat -- go.mod go.sum
```

확인할 것:
- `go vet`과 `go test`가 통과한다.
- `--help`에 `holiday`와 `ua-refresh`가 모두 있다.
- `check 2026-03-02`가 `holiday — 쉬는 날 삼일절`, `check 2026-03-03`이 `workday`를 낸다.
- `status`에 `workdays only`와 `today:` 줄이 있다.
- `go.mod`에 변경이 없다(의존성이 늘지 않았다).

- [ ] **Step 6: 코드 리뷰**

subagent-driven-development로 진행했다면 각 Task의 reviewer로 충분하다. executing-plans나 순수 plan mode로 진행했다면 여기서 `/code-review`를 최소 1회 실행하고, 나온 지적을 고친 뒤 Step 5의 검증을 다시 돌린다.

- [ ] **Step 7: 커밋**

```bash
git add README.md CLAUDE.md docs/superpowers/specs/2026-09-12-holiday-workdays-design.md
git commit -m "docs: document the holiday tool and workdays-only runs

Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>"
```

**Task 8 완료조건:** Step 5의 모든 명령을 실제로 실행해 위 다섯 가지를 눈으로 확인했다. 확인하지 않은 항목이 있으면 이 Task는 끝나지 않은 것이다.

---

## 자체 점검 결과

플랜을 다 쓴 뒤 스펙과 대조한 결과다.

**스펙 항목별 담당 Task**

| 스펙 항목 | Task |
|---|---|
| 판정 규칙(주말 + 목록 + extra − ignore) | 1 |
| 파일 위치(설정과 저장 파일 분리) | 2 |
| 목록 없음·깨짐·범위 밖일 때 주말만 적용하고 경고 | 2 |
| 설정 오류만 오류, 나머지는 경고 | 2 |
| 달력 내려받기, CRLF·접힌 줄·배타적 끝 날짜 | 3 |
| 공휴일 0건이면 저장하지 않음 | 3 |
| 목록이 오늘을 못 덮을 때만 스스로 한 번 회복 | 3(`OpenOrRefresh`), 6(호출) |
| 목록이 멀쩡하면 네트워크를 쓰지 않음 | 3(요청 건수 0을 시험으로 고정) |
| `--dry-run`은 회복하지 않음 | 6 |
| 회복 실패 경고를 Slack 알림과 `last-run.json`에 실음 | 6 |
| 달력 주소를 설정으로 뺌 | 2(설정), 3(사용) |
| `sync`·`list`·`check` 명령 | 4 |
| `workdays_only` 기본값 켜짐 | 5 |
| 게이트 위치와 건너뛸 때 건드리지 않는 것들 | 6 |
| 사람이 직접 실행해도 건너뜀, `--force` | 6 |
| `--dry-run`은 쉬는 날에도 전부 보여줌 | 6 |
| `status`의 오늘 판정 줄 | 7 |
| 시험 방법(네트워크 없이) | 1·2·3·6·7 |

**이름 일관성**: `Entry`·`Calendar`·`New`·`Holiday`·`IsWorkday`·`Covers`·`Paths`·`Config`·`File`·`Range`·`LoadConfig`·`ReadFile`·`WriteFile`·`Open`·`OpenOrRefresh`·`ParseICS`·`Fetch`·`Sync`·`holidayGate`·`todayLine`·`StatusText`가 정의된 곳과 쓰이는 곳에서 같은 철자다. `dateLayout`은 Task 1에서 정의하고 2·3이 쓴다. `RunResult.Warning`은 Task 6에서 정의하고 같은 Task의 `SlackText`가 읽는다.

**`status`는 자동 회복을 하지 않는다**: Task 7의 `todayLine`은 `Open`을 쓴다. 상태를 보여주는 명령이 파일을 바꾸면 놀랍기 때문이며, `--dry-run`과 같은 이유다. 아침 실행(Task 6)만 회복한다.

**스펙에 있으나 플랜에서 형태가 바뀐 것**: 스펙 3절은 설정 템플릿을 `~/.config/gofer/holiday.toml`로만 적고 만드는 방법을 정하지 않았다. 플랜은 `gofer holiday sync --init-config`로 템플릿을 만들게 했다. 설정 파일이 없어도 모든 기능이 동작하므로 이 명령은 편의 수단이다.
