package uarefresh

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"gofer/internal/holiday"
)

// aWeekday 는 테스트 기본값으로 쓰는, 실행 시점의 실제 요일과 무관한 고정된 평일이다.
// workdays_only 가 기본으로 켜지면서, o.Now 를 실제 벽시계에 맡기면 테스트를 우연히
// 토·일에 돌릴 때 기존 테스트가 전부 "쉬는 날"로 건너뛰어져 깨진다.
var aWeekday = time.Date(2026, 9, 9, 7, 30, 0, 0, time.Local)

// slackStub 은 받은 DM 본문을 모으는 가짜 웹훅이다.
type slackStub struct {
	mu     sync.Mutex
	texts  []string
	status int
	srv    *httptest.Server
}

func newSlackStub(t *testing.T, status int) *slackStub {
	t.Helper()
	s := &slackStub{status: status}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.texts = append(s.texts, string(b))
		s.mu.Unlock()
		w.WriteHeader(s.status)
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *slackStub) count() int { s.mu.Lock(); defer s.mu.Unlock(); return len(s.texts) }
func (s *slackStub) last() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.texts) == 0 {
		return ""
	}
	return s.texts[len(s.texts)-1]
}

// setupCommand 는 임시 Paths 와 설정 파일을 만든다. repos 는 (path, trunk) 쌍이다.
func setupCommand(t *testing.T, f fakeClaude, webhook string, repos ...RepoConfig) (CommandOptions, *bytes.Buffer) {
	t.Helper()
	base := t.TempDir()
	paths := Paths{
		Config:   filepath.Join(base, "config", "ua-refresh.toml"),
		LogDir:   filepath.Join(base, "logs"),
		StateDir: filepath.Join(base, "state"),
		Plist:    filepath.Join(base, "gofer.ua-refresh.plist"),
	}
	hpaths := holiday.Paths{
		Config:   filepath.Join(base, "config", "holiday.toml"),
		StateDir: filepath.Join(base, "state", "holiday"),
	}
	if err := holiday.WriteFile(hpaths.Calendar(), holiday.File{
		SyncedAt: aWeekday.Format(time.RFC3339),
		Source:   "https://example.invalid/calendar.ics",
		Covers:   holiday.Range{From: "2000-01-01", To: "2100-12-31"},
		Holidays: []holiday.Entry{{Date: "2000-01-01", Name: "placeholder"}},
	}); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[schedule]\nat = \"07:30\"\n[claude]\nbudget_usd = 5\ntimeout_min = 1\n[notify.slack]\nwebhook_url = %q\n[env]\nextra_path = [%q]\n", webhook, f.Dir)
	for _, r := range repos {
		fmt.Fprintf(&b, "[[repos]]\npath = %q\ntrunk = %q\n", r.Path, r.Trunk)
	}
	os.MkdirAll(filepath.Dir(paths.Config), 0o700)
	if err := os.WriteFile(paths.Config, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	return CommandOptions{Paths: paths, Holiday: hpaths, Stdout: &out, TTY: false, Now: func() time.Time { return aWeekday }}, &out
}

func TestRunCommandEndToEnd(t *testing.T) {
	work, origin := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "ok")
	pushFromClone(t, origin, "main", "a.txt", "a")
	slack := newSlackStub(t, 200)
	o, out := setupCommand(t, f, slack.srv.URL, RepoConfig{Path: work, Trunk: "main"})

	if err := RunCommand(t.Context(), o); err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	last, err := ReadRunResult(o.Paths.LastRun())
	if err != nil || len(last.Repos) != 1 || last.Repos[0].Status != StatusUpdated {
		t.Fatalf("last-run: %+v %v", last, err)
	}
	if slack.count() != 2 || !strings.Contains(slack.last(), "1 updated") || !strings.Contains(slack.last(), "example-api") {
		t.Errorf("slack: %d msgs, last=%q", slack.count(), slack.last())
	}
	logBytes, err := os.ReadFile(last.LogPath)
	if err != nil || !strings.Contains(string(logBytes), "graph updated") || !strings.Contains(string(logBytes), "claude:") {
		t.Errorf("daily log: %v\n%s", err, logBytes)
	}
	if !strings.Contains(out.String(), "done · 1 updated") {
		t.Errorf("stdout:\n%s", out.String())
	}
	if _, err := os.Stat(o.Paths.Lock()); !errors.Is(err, os.ErrNotExist) {
		t.Error("lock should be released")
	}
}

// TestRunCommandSendsStartNotification 은 run 이 시작할 때도 Slack 에 짧은 알림을 보내는지 본다 — 끝날 때
// 보내는 결과 요약 DM 과는 별개로, 시작했다는 사실만 먼저 알린다.
func TestRunCommandSendsStartNotification(t *testing.T) {
	work, _ := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "ok")
	head, _ := Head(t.Context(), work)
	writeMeta(t, work, ".ua", head)
	slack := newSlackStub(t, 200)
	o, _ := setupCommand(t, f, slack.srv.URL, RepoConfig{Path: work, Trunk: "main"})

	if err := RunCommand(t.Context(), o); err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	if slack.count() != 2 {
		t.Fatalf("want 2 slack messages (start + end), got %d: %v", slack.count(), slack.texts)
	}
	first := slack.texts[0]
	if !strings.Contains(first, "starting") || !strings.Contains(first, "1 repo") {
		t.Errorf("first message should be the start notification, got %q", first)
	}
	if !strings.Contains(slack.last(), "up to date") {
		t.Errorf("last message should still be the end summary, got %q", slack.last())
	}
}

func TestRunCommandReturnsIncompleteButStillNotifies(t *testing.T) {
	work, _ := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "ok")
	os.WriteFile(filepath.Join(work, "README.md"), []byte("dirty"), 0o644)
	slack := newSlackStub(t, 200)
	o, _ := setupCommand(t, f, slack.srv.URL, RepoConfig{Path: work, Trunk: "main"})

	err := RunCommand(t.Context(), o)
	if !errors.Is(err, ErrIncomplete) {
		t.Fatalf("want ErrIncomplete, got %v", err)
	}
	if slack.count() != 2 || !strings.Contains(slack.last(), "1 skipped") {
		t.Errorf("slack: %d msgs, last=%q", slack.count(), slack.last())
	}
}

func TestRunCommandFallsBackToOsascriptWhenSlackFails(t *testing.T) {
	work, _ := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "ok")
	head, _ := Head(t.Context(), work)
	writeMeta(t, work, ".ua", head)
	argsFile := installFakeOsascript(t)
	slack := newSlackStub(t, 500)
	webhook := slack.srv.URL + "/services/T/B/SECRETTOKEN"
	o, _ := setupCommand(t, f, webhook, RepoConfig{Path: work, Trunk: "main"})

	if err := RunCommand(t.Context(), o); err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	b, err := os.ReadFile(argsFile)
	if err != nil || !strings.Contains(string(b), "display notification") || !strings.Contains(string(b), "1 up to date") {
		t.Errorf("osascript args: %v\n%s", err, b)
	}
	last, _ := ReadRunResult(o.Paths.LastRun())
	logBytes, _ := os.ReadFile(last.LogPath)
	if !strings.Contains(string(logBytes), "slack:") {
		t.Errorf("log should record the slack failure:\n%s", logBytes)
	}
	if strings.Contains(string(logBytes), "SECRETTOKEN") {
		t.Errorf("daily log must not contain the webhook URL:\n%s", logBytes)
	}
}

func TestRunCommandOnlySelectsOneRepo(t *testing.T) {
	first, _ := newRepoWithOrigin(t, "main")
	second, _ := newRepoWithOrigin(t, "develop")
	second2 := filepath.Join(filepath.Dir(second), "example-web")
	os.Rename(second, second2)
	f := installFakeClaude(t, "ok")
	for _, w := range []string{first, second2} {
		h, _ := Head(t.Context(), w)
		writeMeta(t, w, ".ua", h)
	}
	slack := newSlackStub(t, 200)
	o, _ := setupCommand(t, f, slack.srv.URL, RepoConfig{Path: first, Trunk: "main"}, RepoConfig{Path: second2, Trunk: "develop"})

	o.Only = "example-web"
	if err := RunCommand(t.Context(), o); err != nil {
		t.Fatal(err)
	}
	last, _ := ReadRunResult(o.Paths.LastRun())
	if len(last.Repos) != 1 || last.Repos[0].Name != "example-web" {
		t.Errorf("last-run repos: %+v", last.Repos)
	}

	o.Only = "nope"
	if err := RunCommand(t.Context(), o); err == nil || !strings.Contains(err.Error(), `no repo named "nope"`) {
		t.Errorf("unknown --only: %v", err)
	}
}

func TestDryRunTouchesNothing(t *testing.T) {
	work, origin := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "ok")
	before, _ := Head(t.Context(), work)
	pushFromClone(t, origin, "main", "a.txt", "a")
	slack := newSlackStub(t, 200)
	o, out := setupCommand(t, f, slack.srv.URL, RepoConfig{Path: work, Trunk: "main"})
	o.DryRun = true

	if err := RunCommand(t.Context(), o); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "config OK · 1 repos") || !strings.Contains(out.String(), "then /understand (no graph yet)") {
		t.Errorf("stdout:\n%s", out.String())
	}
	if after, _ := Head(t.Context(), work); after != before {
		t.Error("dry-run must not merge")
	}
	if f.called() || slack.count() != 0 {
		t.Error("dry-run must not call claude or slack")
	}
	for _, p := range []string{o.Paths.Lock(), o.Paths.LastRun(), o.Paths.LogDir} {
		if _, err := os.Stat(p); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("dry-run must not create %s", p)
		}
	}
}

func TestDryRunShowsGuardAndFullDecisions(t *testing.T) {
	skipped, _ := newRepoWithOrigin(t, "main")
	run(t, skipped, "git", "checkout", "-q", "-b", "feature-x")
	full, _ := newRepoWithOrigin(t, "main")
	writeMeta(t, full, ".ua", "0123456789abcdef0123456789abcdef01234567")
	f := installFakeClaude(t, "ok")
	o, out := setupCommand(t, f, "https://hooks.slack.com/services/T/B/X", RepoConfig{Path: skipped, Trunk: "main"}, RepoConfig{Path: full, Trunk: "main"})
	o.DryRun = true
	if err := RunCommand(t.Context(), o); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `skip: branch is "feature-x", expected "main"`) || !strings.Contains(out.String(), "then /understand --full (graph commit 0123456 not in repo)") {
		t.Errorf("stdout:\n%s", out.String())
	}
}

// TestRunCommandCancelKillsClaudeAndStillNotifies 는 "fang 시그널 → ctx 취소 → 프로세스 그룹 종료 → DM" 이음매를
// 끝까지 붙여서 본다(최종 리뷰 M10-b). TTY 없는 실행(launchd 와 같은 조건)이므로 tui.Run 은 ctx 를 직접 보지
// 않고 events 가 닫힐 때까지 기다린다 — 취소는 오케스트레이터 쪽 ctx 전파만으로 끝까지 이어져야 한다.
func TestRunCommandCancelKillsClaudeAndStillNotifies(t *testing.T) {
	work, _ := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "hang")
	slack := newSlackStub(t, 200)
	o, _ := setupCommand(t, f, slack.srv.URL, RepoConfig{Path: work, Trunk: "main"})

	ctx, cancel := context.WithCancel(t.Context())
	errCh := make(chan error, 1)
	go func() { errCh <- RunCommand(ctx, o) }()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(f.Child); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	b, err := os.ReadFile(f.Child)
	if err != nil {
		t.Fatalf("fake claude never started: %v", err)
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))

	cancel()

	select {
	case err := <-errCh:
		if !errors.Is(err, ErrIncomplete) && err == nil {
			t.Errorf("want a non-nil error (ErrIncomplete expected for a non-TTY run), got %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("RunCommand did not return within 15s of cancel")
	}

	killDeadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(killDeadline) {
		if syscall.Kill(pid, 0) == syscall.ESRCH {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if syscall.Kill(pid, 0) != syscall.ESRCH {
		syscall.Kill(pid, syscall.SIGKILL) // 정리
		t.Errorf("grandchild sleep (pid %d) survived cancellation", pid)
	}

	if slack.count() != 2 {
		t.Errorf("slack: want 2 messages (start + end) despite cancellation, got %d", slack.count())
	}
}

func TestRunCommandRefusesWhenLocked(t *testing.T) {
	work, _ := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "ok")
	o, _ := setupCommand(t, f, "https://hooks.slack.com/services/T/B/X", RepoConfig{Path: work, Trunk: "main"})
	release, err := AcquireLock(o.Paths.Lock())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if err := RunCommand(t.Context(), o); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("want ErrAlreadyRunning, got %v", err)
	}
}

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
	// setupCommand 가 심어 둔 자리채우기 달력(2000~2100)은 2026 년도 덮어서, 이 테스트가 바라는
	// "회복 실패" 경로를 타지 않는다. 이 테스트만의 낡은 달력으로 덮어써 오늘을 덮지 못하게 한다.
	if err := holiday.WriteFile(o.Holiday.Calendar(), holiday.File{
		SyncedAt: "2020-01-01T00:00:00Z",
		Source:   "https://example.invalid/calendar.ics",
		Covers:   holiday.Range{From: "2000-01-01", To: "2000-12-31"}, // 2026 을 덮지 못하는 낡은 목록
		Holidays: []holiday.Entry{{Date: "2000-01-01", Name: "placeholder"}},
	}); err != nil {
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

// TestHolidayConfigErrorDoesNotBlockRun 은 holiday.toml 오타처럼 설정 자체가 깨졌을 때도
// 하루 실행이 조용히 멎지 않는지 본다 — 판정 없이 진행하고 문제를 경고로 알려야 한다.
func TestHolidayConfigErrorDoesNotBlockRun(t *testing.T) {
	work, origin := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "ok")
	pushFromClone(t, origin, "main", "a.txt", "a")
	slack := newSlackStub(t, 200)
	o, _ := setupCommand(t, f, slack.srv.URL, RepoConfig{Path: work, Trunk: "main"})

	if err := os.MkdirAll(filepath.Dir(o.Holiday.Config), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(o.Holiday.Config, []byte("extra = [\"not-a-date\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := RunCommand(t.Context(), o); err != nil {
		t.Fatalf("a broken holiday.toml must not block the run: %v", err)
	}
	if slack.count() != 2 {
		t.Fatalf("want 2 slack messages (start + end), got %d", slack.count())
	}
	if !strings.Contains(slack.last(), "holiday config error") {
		t.Errorf("result message should mention the holiday config error, got %q", slack.last())
	}
	last, err := ReadRunResult(o.Paths.LastRun())
	if err != nil || !strings.Contains(last.Warning, "holiday config error") {
		t.Errorf("last-run.json should keep the warning: %v %+v", err, last.Warning)
	}
}

// TestDryRunNeverRefreshesOverNetwork 은 --dry-run 이 저장된 목록이 낡았어도 holiday.Open 만
// 부르고, 네트워크로 회복을 시도하는 holiday.OpenOrRefresh 는 절대 부르지 않는지 본다.
// 다른 dry-run 테스트들이 쓰는 setupCommand 의 자리채우기 달력은 항상 테스트 날짜를 덮으므로,
// 이 보장을 실제로 검증하려면 낡은 달력과 요청을 세는 서버가 따로 필요하다.
func TestDryRunNeverRefreshesOverNetwork(t *testing.T) {
	work, _ := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "ok")
	slack := newSlackStub(t, 200)
	o, out := setupCommand(t, f, slack.srv.URL, RepoConfig{Path: work, Trunk: "main"})

	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	if err := os.MkdirAll(filepath.Dir(o.Holiday.Config), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(o.Holiday.Config, []byte("url = \""+srv.URL+"\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// aWeekday 를 덮지 못하는 낡은 목록으로 덮어써, "목록이 멀쩡해서 회복을 안 한다"가 아니라
	// "dry-run 이라서 애초에 회복을 시도하지 않는다"를 검증한다.
	if err := holiday.WriteFile(o.Holiday.Calendar(), holiday.File{
		SyncedAt: "2000-01-01T00:00:00Z",
		Source:   "https://example.invalid/calendar.ics",
		Covers:   holiday.Range{From: "2000-01-01", To: "2000-12-31"},
		Holidays: []holiday.Entry{{Date: "2000-01-01", Name: "placeholder"}},
	}); err != nil {
		t.Fatal(err)
	}
	o.DryRun = true

	if err := RunCommand(t.Context(), o); err != nil {
		t.Fatal(err)
	}
	if hits != 0 {
		t.Errorf("--dry-run must use holiday.Open (network-free), not OpenOrRefresh; got %d requests", hits)
	}
	if !strings.Contains(out.String(), "config OK") {
		t.Errorf("stdout should still show the dry-run plan:\n%s", out.String())
	}
}

// TestDryRunOnDayOffStillShowsPlan 은 쉬는 날에도 --dry-run 이 계획을 생략하지 않고,
// "건너뛴다"는 안내와 평소의 dry-run 출력을 함께 보여주는지 본다.
func TestDryRunOnDayOffStillShowsPlan(t *testing.T) {
	work, _ := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "ok")
	slack := newSlackStub(t, 200)
	o, out := setupCommand(t, f, slack.srv.URL, RepoConfig{Path: work, Trunk: "main"})
	o.Now = func() time.Time { return time.Date(2026, 9, 12, 7, 30, 0, 0, time.Local) } // 토요일
	o.DryRun = true

	if err := RunCommand(t.Context(), o); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "a real run would skip today") {
		t.Errorf("stdout should note that a real run would skip today:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "config OK") {
		t.Errorf("stdout should still show the full dry-run plan:\n%s", out.String())
	}
	if slack.count() != 0 {
		t.Errorf("dry-run must not send slack messages, got %d", slack.count())
	}
}
