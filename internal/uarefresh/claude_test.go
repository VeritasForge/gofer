package uarefresh

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRunClaudeArgsEnvAndCwd(t *testing.T) {
	f := installFakeClaude(t, "noop")
	repo := t.TempDir()
	res, err := RunClaude(t.Context(), ClaudeOptions{
		Dir: repo, Full: true, BudgetUSD: 20, Timeout: 10 * time.Second, Model: "example-model",
		ExtraPath: []string{f.Dir}, OAuthToken: "tok-123",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalCostUSD != 0.1 || res.IsError {
		t.Errorf("result = %+v", res)
	}
	want := []string{"-p", "/understand --full", "--dangerously-skip-permissions", "--output-format", "json", "--max-budget-usd", "20", "--model", "example-model"}
	if got := f.argsLines(t); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("args = %q", got)
	}
	env, _ := os.ReadFile(f.Env)
	if !strings.HasPrefix(string(env), "PATH="+f.Dir+string(os.PathListSeparator)) || !strings.Contains(string(env), "CLAUDE_CODE_OAUTH_TOKEN=tok-123") {
		t.Errorf("env = %q", env)
	}
	cwd, _ := os.ReadFile(f.Cwd)
	wantCwd, _ := filepath.EvalSymlinks(repo)
	if strings.TrimSpace(string(cwd)) != wantCwd {
		t.Errorf("cwd = %q want %q", cwd, wantCwd)
	}
}

func TestRunClaudeWithoutOptionalFlags(t *testing.T) {
	f := installFakeClaude(t, "noop")
	_, err := RunClaude(t.Context(), ClaudeOptions{Dir: t.TempDir(), BudgetUSD: 7.5, Timeout: 10 * time.Second, ExtraPath: []string{f.Dir}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-p", "/understand", "--dangerously-skip-permissions", "--output-format", "json", "--max-budget-usd", "7.5"}
	if got := f.argsLines(t); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("args = %q", got)
	}
	env, _ := os.ReadFile(f.Env)
	if !strings.Contains(string(env), "CLAUDE_CODE_OAUTH_TOKEN=\n") {
		t.Errorf("token should be empty, env = %q", env)
	}
}

func TestRunClaudeFindsBinaryViaExtraPathOnly(t *testing.T) {
	f := installFakeClaude(t, "noop")
	t.Setenv("PATH", "/nonexistent")
	if _, err := RunClaude(t.Context(), ClaudeOptions{Dir: t.TempDir(), BudgetUSD: 1, Timeout: 10 * time.Second, ExtraPath: []string{f.Dir}}); err != nil {
		t.Fatal(err)
	}
	if _, err := RunClaude(t.Context(), ClaudeOptions{Dir: t.TempDir(), BudgetUSD: 1, Timeout: 10 * time.Second}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("want not-found error, got %v", err)
	}
}

func TestRunClaudeReportsIsError(t *testing.T) {
	f := installFakeClaude(t, "error")
	res, err := RunClaude(t.Context(), ClaudeOptions{Dir: t.TempDir(), BudgetUSD: 1, Timeout: 10 * time.Second, ExtraPath: []string{f.Dir}})
	if err == nil || !strings.Contains(err.Error(), "budget exceeded") {
		t.Fatalf("want is_error, got %v", err)
	}
	if res.TotalCostUSD != 0.2 {
		t.Errorf("cost should still be parsed: %+v", res)
	}
}

func TestRunClaudeRejectsNonJSON(t *testing.T) {
	f := installFakeClaude(t, "garbage")
	_, err := RunClaude(t.Context(), ClaudeOptions{Dir: t.TempDir(), BudgetUSD: 1, Timeout: 10 * time.Second, ExtraPath: []string{f.Dir}})
	if err == nil || !strings.Contains(err.Error(), "not JSON") {
		t.Fatalf("want JSON error, got %v", err)
	}
}

func TestRunClaudeTimeoutKillsProcessGroup(t *testing.T) {
	f := installFakeClaude(t, "hang")
	start := time.Now()
	res, err := RunClaude(t.Context(), ClaudeOptions{Dir: t.TempDir(), BudgetUSD: 1, Timeout: 500 * time.Millisecond, ExtraPath: []string{f.Dir}})
	if err == nil || !res.TimedOut {
		t.Fatalf("want timeout, got %v %+v", err, res)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatalf("took %s; group kill did not work", time.Since(start))
	}
	b, err := os.ReadFile(f.Child)
	if err != nil {
		t.Fatal("child pid not recorded:", err)
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); err == syscall.ESRCH {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	syscall.Kill(pid, syscall.SIGKILL) // 정리
	t.Fatalf("grandchild sleep (pid %d) survived the process-group kill", pid)
}
