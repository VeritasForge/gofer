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
)

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
	return CommandOptions{Paths: paths, Stdout: &out, TTY: false}, &out
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
	if slack.count() != 1 || !strings.Contains(slack.last(), "1 updated") || !strings.Contains(slack.last(), "example-api") {
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
	if slack.count() != 1 || !strings.Contains(slack.last(), "1 skipped") {
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

	if slack.count() != 1 {
		t.Errorf("slack: want 1 message despite cancellation, got %d", slack.count())
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
