package uarefresh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeClaude 는 PATH 에 넣을 가짜 claude 다. 호출 인자·환경·cwd 를 파일에 남기고 모드대로 동작한다.
type fakeClaude struct {
	Dir   string // 스크립트 디렉토리 — ClaudeOptions.ExtraPath 로 넘긴다
	Args  string // 인자 기록 (한 줄에 하나)
	Env   string // PATH 와 토큰 기록
	Cwd   string // 실행 디렉토리 기록
	Child string // hang 모드가 띄운 sleep 의 pid
}

// installFakeClaude 는 가짜 claude 스크립트를 만든다.
//   ok      그래프 meta.json 의 해시를 HEAD 로 갱신하고 성공 JSON 출력 (실제 /understand 성공과 같은 효과)
//   noop    성공 JSON 만 출력, 그래프는 건드리지 않음 (해시 검증 실패 경로)
//   error   is_error:true JSON 출력 후 exit 1
//   hang    sleep 300 을 백그라운드로 띄우고 기다림 (타임아웃·그룹 종료 검증)
//   garbage JSON 이 아닌 출력
func installFakeClaude(t *testing.T, mode string) fakeClaude {
	t.Helper()
	dir := t.TempDir()
	f := fakeClaude{
		Dir: dir, Args: filepath.Join(dir, "args"), Env: filepath.Join(dir, "env"),
		Cwd: filepath.Join(dir, "cwd"), Child: filepath.Join(dir, "child"),
	}
	t.Setenv("FAKE_CLAUDE_MODE", mode)
	t.Setenv("FAKE_CLAUDE_DIR", dir)
	script := `#!/bin/sh
printf '%s\n' "$@" > "$FAKE_CLAUDE_DIR/args"
printf 'PATH=%s\nCLAUDE_CODE_OAUTH_TOKEN=%s\n' "$PATH" "$CLAUDE_CODE_OAUTH_TOKEN" > "$FAKE_CLAUDE_DIR/env"
pwd -P > "$FAKE_CLAUDE_DIR/cwd"
case "$FAKE_CLAUDE_MODE" in
  ok)
    d=.ua; [ -d .understand-anything ] && d=.understand-anything
    mkdir -p "$d"
    printf '{"gitCommitHash":"%s"}' "$(git rev-parse HEAD)" > "$d/meta.json"
    echo '{"type":"result","is_error":false,"total_cost_usd":0.5,"result":"done"}' ;;
  noop)    echo '{"type":"result","is_error":false,"total_cost_usd":0.1,"result":"done"}' ;;
  error)   echo '{"type":"result","is_error":true,"total_cost_usd":0.2,"result":"budget exceeded"}'; exit 1 ;;
  hang)    sleep 300 & echo $! > "$FAKE_CLAUDE_DIR/child"; wait ;;
  garbage) echo 'not json' ;;
esac
`
	if err := os.WriteFile(filepath.Join(dir, "claude"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f fakeClaude) called() bool {
	_, err := os.Stat(f.Args)
	return err == nil
}

func (f fakeClaude) argsLines(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile(f.Args)
	if err != nil {
		t.Fatalf("fake claude was not called: %v", err)
	}
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}
