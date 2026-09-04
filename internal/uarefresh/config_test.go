package uarefresh

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// 유효한 설정 한 벌을 임시 디렉토리에 만든다. repo 는 .git 디렉토리만 있는 빈 껍데기다.
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	repo := filepath.Join(dir, "example-api")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	body = strings.ReplaceAll(body, "{{REPO}}", repo)
	path := filepath.Join(dir, "ua-refresh.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

const validConfig = `
[schedule]
at = "07:30"
[claude]
budget_usd = 20
timeout_min = 60
[notify.slack]
webhook_url = "https://hooks.slack.com/services/T/B/X"
[env]
extra_path = []
[[repos]]
path = "{{REPO}}"
trunk = "main"
`

func TestLoadValidConfig(t *testing.T) {
	cfg, err := Load(writeConfig(t, validConfig))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Claude.BudgetUSD != 20 || cfg.Claude.Timeout() != 60*time.Minute {
		t.Errorf("claude section: %+v", cfg.Claude)
	}
	if len(cfg.Repos) != 1 || cfg.Repos[0].Name() != "example-api" || cfg.Repos[0].Trunk != "main" {
		t.Errorf("repos: %+v", cfg.Repos)
	}
}

func TestLoadCollectsAllErrors(t *testing.T) {
	path := writeConfig(t, `
[schedule]
at = "7:30pm"
[claude]
budget_usd = 0
timeout_min = 0
[notify.slack]
webhook_url = ""
[[repos]]
path = "/nonexistent/repo"
trunk = ""
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"schedule.at", "budget_usd", "timeout_min", "webhook_url", "repos[0].path", "repos[0].trunk"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q, got:\n%v", want, err)
		}
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	// 최상위에 오타 키를 둔다 ([claude] 를 다시 열면 TOML 자체가 "table already defined" 로 실패하므로)
	path := writeConfig(t, "budgett = 1\n"+validConfig)
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "unknown key") {
		t.Fatalf("want unknown key error, got %v", err)
	}
}

func TestLoadRequiresAtLeastOneRepo(t *testing.T) {
	path := writeConfig(t, strings.Split(validConfig, "[[repos]]")[0])
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "repos") {
		t.Fatalf("want repos error, got %v", err)
	}
}

func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	if got := ExpandHome("~/src/x"); got != filepath.Join(home, "src/x") {
		t.Errorf("got %q", got)
	}
	if got := ExpandHome("/abs"); got != "/abs" {
		t.Errorf("got %q", got)
	}
}

func TestResolveExtraPathPicksLastGlobMatch(t *testing.T) {
	dir := t.TempDir()
	for _, v := range []string{"v18.0.0", "v20.1.0", "v9.9.9"} {
		if err := os.MkdirAll(filepath.Join(dir, "node", v, "bin"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got := ResolveExtraPath([]string{
		filepath.Join(dir, "node", "*", "bin"),
		filepath.Join(dir, "missing", "*"),
		"/usr/bin",
	})
	want := []string{filepath.Join(dir, "node", "v9.9.9", "bin"), "/usr/bin"} // 이름순 마지막 (설계 3절)
	if strings.Join(got, ":") != strings.Join(want, ":") {
		t.Errorf("got %v want %v", got, want)
	}
}

func TestWriteTemplateRefusesToOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg", "ua-refresh.toml")
	if err := WriteTemplate(path); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Errorf("perm = %o", info.Mode().Perm())
	}
	if err := os.WriteFile(path, []byte("# mine\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := WriteTemplate(path)
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("want ErrExist, got %v", err)
	}
	b, _ := os.ReadFile(path)
	if string(b) != "# mine\n" {
		t.Errorf("existing file was modified: %q", b)
	}
}

func TestPaths(t *testing.T) {
	p := Paths{LogDir: "/l", StateDir: "/s"}
	if got := p.DailyLog(time.Date(2026, 9, 5, 7, 30, 0, 0, time.Local)); got != "/l/2026-09-05.log" {
		t.Errorf("DailyLog = %q", got)
	}
	if p.Lock() != "/s/run.lock" || p.LastRun() != "/s/last-run.json" || p.LaunchdLog() != "/l/launchd.log" {
		t.Errorf("paths: %+v", p)
	}
}
