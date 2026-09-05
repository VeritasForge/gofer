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

// UnattendedSystemPrompt 는 launchd 무인 실행에서 /understand 가 사용자 확인을 기다리지 않게 하는 시스템 프롬프트 추가분이다.
// 스킬은 .understandignore 확인과 100개 초과 파일 게이트에서 "confirm" 을 기다리는데(-p 모드에서는 답할 사람이 없다),
// 그 자리에서 기본값으로 진행하라고 못 박는다. 대시보드 같은 장기 실행 서버도 금지한다.
// 문장 사이에 실제 개행을 넣지 않는다 — 인자 하나가 여러 줄이면 가짜 claude 스크립트(fakeclaude_test.go)가
// 인자를 줄 단위로 기록하는 방식과 어긋나 하나의 인자가 여러 줄로 쪼개져 기록된다.
const UnattendedSystemPrompt = "This is an unattended, non-interactive run started by a scheduler; no human can reply. " +
	"Never stop to ask a question or wait for confirmation. Whenever a skill or instruction says to ask the user, " +
	"confirm, or wait, treat the answer as \"confirm — proceed with the defaults\": keep the existing .understandignore as is, " +
	"accept the changed-file count however large, use the available budget, and continue the incremental update " +
	"until knowledge-graph.json and meta.json are written. Do not launch the dashboard or any long-running server. " +
	"If the graph is already up to date at this commit, do nothing and exit."

// ClaudeArgs 는 실행 인자다. --bare 는 플러그인 스킬을 건너뛰므로 쓰지 않는다 (설계 2절).
// --append-system-prompt 로 UnattendedSystemPrompt 를 붙여 /understand 의 확인 대기 게이트를 무인 실행에 맞춘다.
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
		"--append-system-prompt", UnattendedSystemPrompt,
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
	if errors.Is(ctx.Err(), context.Canceled) {
		return res, fmt.Errorf("claude cancelled")
	}
	if runErr != nil {
		if len(bytes.TrimSpace(out)) == 0 {
			return res, fmt.Errorf("claude produced no output: %w", runErr)
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
