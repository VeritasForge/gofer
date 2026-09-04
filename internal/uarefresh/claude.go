package uarefresh

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// ClaudeOptions 는 `claude -p "/understand"` 한 번의 실행 조건이다 (설계 4절 "Claude 호출 세부").
type ClaudeOptions struct {
	Dir        string
	Full       bool
	BudgetUSD  float64
	Timeout    time.Duration
	Model      string
	ExtraPath  []string
	OAuthToken string
	Stderr     io.Writer
}

// ClaudeResult 는 --output-format json 결과 중 쓰는 필드와 프로세스 상태다.
type ClaudeResult struct {
	IsError      bool    `json:"is_error"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Result       string  `json:"result"`
	TimedOut     bool    `json:"-"`
}

// killGrace 는 SIGTERM 뒤 SIGKILL 까지 주는 유예다.
const killGrace = 10 * time.Second

// ClaudeArgs 는 실행 인자다. --bare 는 플러그인 스킬을 건너뛰므로 쓰지 않는다 (설계 2절).
func ClaudeArgs(o ClaudeOptions) []string {
	prompt := "/understand"
	if o.Full {
		prompt += " --full"
	}
	args := []string{
		"-p", prompt,
		"--dangerously-skip-permissions",
		"--output-format", "json",
		"--max-budget-usd", strconv.FormatFloat(o.BudgetUSD, 'f', -1, 64),
	}
	if o.Model != "" {
		args = append(args, "--model", o.Model)
	}
	return args
}

// pathEnv 는 extra_path 를 현재 PATH 앞에 붙인 값이다. launchd 의 PATH 는 거의 비어 있다.
func pathEnv(extra []string) string {
	parts := append(slices.Clone(extra), os.Getenv("PATH"))
	return strings.Join(parts, string(os.PathListSeparator))
}

// lookPath 는 주어진 PATH 문자열에서 실행 파일을 찾는다. exec.LookPath 는 프로세스 PATH 만 보므로 쓸 수 없다.
func lookPath(name, path string) (string, error) {
	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			continue
		}
		p := filepath.Join(dir, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
			return p, nil
		}
	}
	return "", fmt.Errorf("%s not found in PATH %q", name, path)
}

// envWithout 은 현재 환경에서 주어진 키를 뺀 사본이다.
func envWithout(keys ...string) []string {
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		if !slices.Contains(keys, k) {
			env = append(env, kv)
		}
	}
	return env
}

// RunClaude 는 claude 를 레포 루트에서 실행하고 JSON 을 파싱한다. 시간 상한을 넘기면 프로세스 그룹 전체에
// SIGTERM, killGrace 뒤 SIGKILL 을 보낸다 — /understand 가 여러 서브셸을 띄우므로 그룹 단위가 아니면 고아가 남는다.
// 오류를 돌려줄 때도 파싱된 비용은 결과에 남긴다.
func RunClaude(ctx context.Context, o ClaudeOptions) (ClaudeResult, error) {
	path := pathEnv(o.ExtraPath)
	bin, err := lookPath("claude", path)
	if err != nil {
		return ClaudeResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, o.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, ClaudeArgs(o)...)
	cmd.Dir = o.Dir
	cmd.Env = append(envWithout("PATH", "CLAUDE_CODE_OAUTH_TOKEN"), "PATH="+path)
	if o.OAuthToken != "" {
		cmd.Env = append(cmd.Env, "CLAUDE_CODE_OAUTH_TOKEN="+o.OAuthToken)
	}
	cmd.Stderr = o.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		pgid := cmd.Process.Pid // Setpgid 로 자식이 그룹 리더이므로 pgid == pid
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
		go func() {
			time.Sleep(killGrace)
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		}()
		return nil
	}
	cmd.WaitDelay = killGrace + 5*time.Second // 손자가 stdout 을 잡고 있어도 Wait 가 돌아오게

	out, runErr := cmd.Output()
	var res ClaudeResult
	if len(bytes.TrimSpace(out)) > 0 {
		if jerr := json.Unmarshal(out, &res); jerr != nil {
			return res, fmt.Errorf("claude output is not JSON: %.200s", out)
		}
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		res.TimedOut = true
		return res, fmt.Errorf("claude timed out after %s", o.Timeout)
	}
	if runErr != nil {
		if len(bytes.TrimSpace(out)) == 0 {
			return res, fmt.Errorf("claude output is not JSON: (empty) %w", runErr)
		}
		if res.IsError {
			return res, fmt.Errorf("claude reported is_error: %.200s", res.Result)
		}
		return res, fmt.Errorf("claude exited: %w", runErr)
	}
	if res.IsError {
		return res, fmt.Errorf("claude reported is_error: %.200s", res.Result)
	}
	return res, nil
}
