# gofer ua-refresh 구현 플랜

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 여러 git 레포의 Understand-Anything 지식 그래프를 매일 아침 launchd 로 자동 갱신하고 결과를 Slack DM 한 통으로 받는 `gofer ua-refresh` 도구를 만든다.

**Architecture:** 바이너리 하나(`gofer`)에 `ua-refresh` 가 서브커맨드로 붙는다. `cmd/` 는 플래그 파싱만 하고 로직은 `internal/uarefresh/` 에 둔다. 오케스트레이터는 화면을 모르고 `tui.Event` 만 채널로 발행하며, 터미널이면 bubbletea 화면이, 아니면(launchd) 한 줄 로그가 같은 Event 를 소비한다. 레포 처리는 순차이며, Claude 호출 전에 그래프 커밋 해시와 HEAD 를 비교해 변경 없는 레포는 비용 0 으로 건너뛴다.

**Tech Stack:** Go 1.26 · cobra + fang · bubbletea/v2 + bubbles/v2 + lipgloss/v2 · BurntSushi/toml · macOS launchd · Slack Incoming Webhook

**Spec:** `docs/superpowers/specs/2026-09-04-ua-refresh-design.md` (이 플랜은 설계 문서의 절 번호로 근거를 댄다. 실행자는 둘 다 읽는다.)

## Global Constraints

- Go `1.26`, 모듈 하나 `module gofer`, 바이너리 하나 `gofer <도구> <동작>` (설계 5절).
- 직접 의존성은 정확히 아래 6개. **Charm v2 계열의 실제 모듈 경로는 `charm.land/...` 이다** — `github.com/charmbracelet/bubbletea/v2` 로 `go get` 하면 "module declares its path as: charm.land/bubbletea/v2" 로 거부된다(플랜 작성 시 Go 모듈 프록시에서 확인, 설계 5절 표도 이에 맞춰 수정함).

  | 역할 | import 경로 | 버전 |
  |---|---|---|
  | CLI 프레임워크 | `github.com/spf13/cobra` | v1.10.2 |
  | help·에러·version·완성 스타일링 | `github.com/charmbracelet/fang` | v1.0.0 |
  | 실행 화면 엔진 | `charm.land/bubbletea/v2` | v2.0.9 |
  | 스피너 | `charm.land/bubbles/v2` | v2.2.1 |
  | 색 | `charm.land/lipgloss/v2` | v2.0.6 |
  | 설정 파싱 | `github.com/BurntSushi/toml` | v1.6.0 |

  다른 모듈을 직접 import 하지 않는다. 예: TTY 판별은 `golang.org/x/term` 대신 `os.File.Stat()` 의 `os.ModeCharDevice` 로, ANSI 제거는 테스트 안의 정규식으로 한다.
- **조직 중립**: 코드·주석·테스트·문서 어디에도 특정 회사·팀·레포·브랜치·워크스페이스 정보를 쓰지 않는다. 예제는 `~/src/example-api`, `main` 처럼 쓴다. 실제 값은 `~/.config/gofer/ua-refresh.toml` 에만 있다 (CLAUDE.md, 설계 1·9절).
- `cmd/` 는 플래그 파싱만, 로직은 `internal/<도구>/`. 오케스트레이터는 화면을 모르고 Event 만 발행한다 (설계 5절).
- `internal/tui`, `internal/slack` 은 도구 공용 — ua-refresh 전용 문구·타입을 넣지 않는다 (설계 5절).
- `go test ./...` 와 `go vet ./...` 가 통과해야 한다. 외부 프로세스(`git`, `claude`, `launchctl`, `osascript`)는 임시 레포와 PATH 앞의 가짜 실행 파일로 테스트한다 (CLAUDE.md, 설계 8절).
- Claude 호출: `claude -p "/understand[ --full]" --dangerously-skip-permissions --output-format json --max-budget-usd N [--model M]`, cwd = 레포 루트. `--bare` 금지. 성공 판정은 종료 코드가 아니라 **그래프 해시 == HEAD**. 타임아웃은 프로세스 그룹 단위 종료 (설계 2·4절).
- 파일 위치 (설계 3절): 설정 `~/.config/gofer/ua-refresh.toml`(0600) · 로그 `~/Library/Logs/gofer/ua-refresh/YYYY-MM-DD.log`, `launchd.log` · 상태 `~/Library/Application Support/gofer/ua-refresh/run.lock`, `last-run.json` · plist `~/Library/LaunchAgents/gofer.ua-refresh.plist` (label `gofer.ua-refresh`).
- 커밋 메시지는 Conventional Commits(`feat:`, `test:`, `chore:`, `docs:`). 커밋마다 `go test ./... && go vet ./...` 통과 상태여야 한다.
- 사용자 대면 문자열(CLI help, 화면, DM)은 영어, 주석은 한국어(설계 문서·CLAUDE.md 와 같은 언어). DM 의 로그 줄 라벨은 설계 6절대로 `로그:`.

---

## 플랜 전체 완료조건

| # | 조건 | 검증 |
|---|---|---|
| A | 모든 Task 의 체크박스가 채워졌다 | 이 파일에서 `- [ ]` 가 0개 |
| B | 단위 테스트·vet 통과 | `go test ./... && go vet ./...` 종료 코드 0 |
| C | 직접 의존성이 위 표의 6개뿐 | `go list -m -f '{{if not .Indirect}}{{.Path}}{{end}}' all \| grep -v '^gofer$' \| wc -l` → `6` |
| D | 조직 중립 | `git grep -n -i -E '<회사명 후보>' -- . ':!docs/superpowers/plans'` 가 비어 있음 (실행자는 `~/.config/gofer/ua-refresh.toml` 의 `path`·`trunk` 값들을 검색어로 삼는다. 값 자체를 플랜에 적지 않는다) |
| E | 통합 검증(Task 15) 완료: `config init` 이 기존 파일 보존 → `run --dry-run` → `run --only <레포>` → `run` → `install` → 다음 날 DM 수신 | Task 15 체크박스 + `~/Library/Application Support/gofer/ua-refresh/last-run.json` 존재 |

## 사전 확인 (플랜 작성 시 실측, 2026-09-04)

- 레포는 비어 있다(`CLAUDE.md`, `docs/` 만 있음, `go.mod` 없음). 아래 "파일 구조" 의 모든 경로는 새로 만든다.
- `~/.config/gofer/ua-refresh.toml` 이 이미 있고(0600) 키 구조가 설계 3절 스키마와 같다. `webhook_url` 만 비어 있다 — Task 15(통합 검증) 전에 사용자가 채운다. 단위 테스트는 가짜 웹훅(httptest)을 쓰므로 그 전 Task 들은 영향이 없다.
- `claude` = `~/.local/bin/claude` → 2.1.260. `--max-budget-usd`, `--output-format`, `--model`, `--dangerously-skip-permissions`, `-p`, `setup-token` 모두 `claude --help` 에 있음.
- `/understand` 스킬(플러그인 2.9.4, `~/.claude/plugins/cache/understand-anything/understand-anything/2.9.4/skills/understand/SKILL.md`) Phase 0 결정표: `--full` → 전체 / 그래프·meta 없음 → 전체(플래그 불필요) / 그래프 있고 해시 같음 → **사용자에게 질문** / 해시 다름 → 증분. 데이터 디렉토리는 `.understand-anything/` 이 있으면 그것, 없으면 `.ua/`. 해시 필드는 `meta.json` 의 `gitCommitHash`.
- `go1.26.7 darwin/arm64`, `git 2.50.1`, `just` 는 `/opt/homebrew/bin/just`, node 는 `~/.nvm/versions/node/v24.18.0/bin`.
- 위 6개 모듈로 `tea.Model`(`View() tea.View`) + `spinner` + `lipgloss` + `toml.DecodeFile` + `fang.Execute` 를 쓰는 샘플이 **빌드됨을 확인**했다. `fang.Execute` 는 `SilenceUsage`/`SilenceErrors` 를 스스로 켠다. `spinner.MiniDot` 의 0번 프레임은 `⠋`. bubbletea v2 는 종료 시 마지막 프레임을 flush 하므로 화면의 요약이 남는다. `Program.Send` 는 종료 후 no-op 이라 안전하다. BurntSushi/toml 은 정수 `20` 을 `float64` 필드로 받는다.

## 파일 구조

```
gofer/
├── go.mod · go.sum · main.go · justfile · .gitignore
├── cmd/
│   ├── root.go                     gofer 루트 커맨드 (도구 등록 한 줄씩)
│   ├── root_test.go
│   └── uarefresh/
│       ├── uarefresh.go            `ua-refresh` 부모 커맨드 + 서브커맨드 등록
│       ├── config.go               config init
│       ├── run.go                  run --dry-run --only
│       ├── install.go              install / uninstall
│       ├── status.go               status
│       └── log.go                  log --follow
├── internal/
│   ├── uarefresh/
│   │   ├── paths.go                파일 위치 (Paths)
│   │   ├── config.go               설정 타입·Load·Validate·템플릿·extra_path 해석
│   │   ├── git.go                  git 래퍼 (브랜치·미커밋·fetch·ff-merge·카운트·해시 존재)
│   │   ├── graph.go                그래프 디렉토리·meta.json 해시·결정(최신/증분/전체)
│   │   ├── claude.go               claude 실행 (PATH 조립, 프로세스 그룹 타임아웃, JSON 파싱)
│   │   ├── result.go               RepoResult·RunResult·Counts·ExitCode·last-run.json 읽기/쓰기
│   │   ├── report.go               Slack DM 본문
│   │   ├── lock.go                 run.lock (pid, stale 판정)
│   │   ├── notify.go               osascript 알림
│   │   ├── orchestrator.go         레포 순차 처리 + Event 발행
│   │   ├── command.go              run 커맨드 로직 (설정→잠금→로그→화면→DM→last-run→종료 코드), dry-run
│   │   ├── launchd.go              plist 생성, install/uninstall/installed
│   │   ├── status.go               status 본문
│   │   ├── logcmd.go               오늘 로그 출력·follow
│   │   └── *_test.go (+ gittest_test.go, fakeclaude_test.go 헬퍼)
│   ├── tui/
│   │   ├── event.go                Event·Kind·Outcome·Item·Labels
│   │   ├── tally.go                집계(카운트·비용) 문구
│   │   ├── plain.go                한 줄 로그 소비자
│   │   ├── model.go                bubbletea 모델 (Render 로 문자열 골든 테스트)
│   │   ├── run.go                  TTY 판별 + 소비자 선택
│   │   └── *_test.go
│   └── slack/
│       ├── webhook.go              Incoming Webhook POST
│       └── webhook_test.go
└── docs/ (설계·플랜·인계)
```

## Task 공통 규칙

- **스킬**: 코드 Task 는 `superpowers:test-driven-development` 로 진행한다(실패 테스트 → 최소 구현 → 통과 → 커밋). 커밋 전 `superpowers:verification-before-completion`. 테스트 실패·예상 밖 동작은 `/demiurge:debug`.
- **Task 별 검증**: 각 Task 의 마지막 단계는 `/demiurge:rl` 로 그 Task 의 완료조건을 검증하는 것이다. `/demiurge:rl` 은 고정 상태 파일을 쓰므로 **동시에 두 개를 돌리지 않는다**(병렬 실행 도구를 쓰더라도 검증은 순차).
- **코드 리뷰**: subagent-driven-development 로 실행하면 Task 리뷰어가 내장돼 있어 그걸로 충분하다. executing-plans 로 실행하면 Task 9(오케스트레이터)와 Task 12(run 커맨드) 완료 후 각각 `/code-review` 를 1회 돌린다.
- **진행 추적**: 이 파일의 체크박스를 갱신한다.
- **테스트 격리**: git 을 쓰는 테스트는 사용자 전역 git 설정(서명 등)에 영향받지 않도록 `GIT_CONFIG_GLOBAL` 을 임시 파일로, `GIT_CONFIG_NOSYSTEM=1` 을 `t.Setenv` 로 둔다(Task 5 헬퍼가 담당). `t.Setenv` 를 쓰므로 그 테스트들은 `t.Parallel()` 을 쓰지 않는다.

---

### Task 1: 인증 실험 — launchd 에서 `claude -p` 가 되는가

**Files:**
- Create (스크래치, git 밖): `/private/tmp/claude-501/-Users-jaeyoungcho-lab-gofer/30f5b3fa-4072-497d-a3f4-93ddd59b9d16/scratchpad/gofer.auth-probe.plist`
- Modify: `docs/autopilot/ua-refresh/HANDOFF.md` (8절 "환경 사실" 에 결과 한 줄)
- Modify (필요 시): `~/.config/gofer/ua-refresh.toml` 의 `oauth_token`

**Interfaces:**
- Consumes: 없음
- Produces: 사실 하나 — "launchd 에서 Keychain 인증이 된다/안 된다". 안 되면 설정의 `oauth_token` 이 채워져 있어야 한다. Task 7 의 `CLAUDE_CODE_OAUTH_TOKEN` 전달 코드는 어느 쪽이든 만든다(설계 2절).

**스킬**: 없음. launchd 동작이 헷갈리면 `man launchd.plist`, Claude Code 인증 관련은 `claude-code-guide` 에이전트.

**완료조건**: 프로브 로그에 `"is_error":false` 를 포함한 JSON 한 덩어리가 있고, 사용한 인증 경로(Keychain 또는 토큰)가 HANDOFF.md 8절에 적혀 있다. 검증: `grep -c '"is_error":false' <프로브 로그>` → `1`.

- [x] **Step 1: 프로브 plist 작성**

`$HOME` 을 실제 홈 경로로 치환해 스크래치 디렉토리에 저장한다. `RunAtLoad` 로 등록 즉시 1회 실행된다. PATH 는 launchd 기본값이 비어 있으므로 명시한다.

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>gofer.auth-probe</string>
	<key>ProgramArguments</key>
	<array>
		<string>$HOME/.local/bin/claude</string>
		<string>-p</string>
		<string>reply ok</string>
		<string>--output-format</string>
		<string>json</string>
	</array>
	<key>EnvironmentVariables</key>
	<dict>
		<key>PATH</key>
		<string>$HOME/.local/bin:/usr/bin:/bin</string>
	</dict>
	<key>RunAtLoad</key>
	<true/>
	<key>StandardOutPath</key>
	<string>$SCRATCH/auth-probe.out.log</string>
	<key>StandardErrorPath</key>
	<string>$SCRATCH/auth-probe.err.log</string>
</dict>
</plist>
```

- [x] **Step 2: 등록 → 결과 확인 → 해제** (Keychain 인증 성공, `"is_error":false`, 비용 $0.83)

```bash
SCRATCH=/private/tmp/claude-501/-Users-jaeyoungcho-lab-gofer/30f5b3fa-4072-497d-a3f4-93ddd59b9d16/scratchpad
launchctl bootstrap gui/$(id -u) "$SCRATCH/gofer.auth-probe.plist"
sleep 30
cat "$SCRATCH/auth-probe.out.log" "$SCRATCH/auth-probe.err.log"
launchctl bootout gui/$(id -u)/gofer.auth-probe
```

Expected(성공): out.log 에 `{"type":"result", ... "is_error":false, ... "total_cost_usd":0.0..., ...}`.

- [x] **Step 3: 실패했으면 토큰 경로로 전환** (해당 없음 — Step 2 성공)

err.log 에 로그인/인증 오류가 있으면:

```bash
claude setup-token      # 브라우저 로그인 → 토큰 문자열 출력
```

출력된 토큰을 `~/.config/gofer/ua-refresh.toml` 의 `[claude] oauth_token = "..."` 에 넣고, 프로브 plist 의 `EnvironmentVariables` 에 `<key>CLAUDE_CODE_OAUTH_TOKEN</key><string>토큰</string>` 을 추가해 Step 2 를 다시 한다. 성공하면 plist 에서 토큰 줄을 지운다(스크래치라도 남기지 않는다).

- [x] **Step 4: 결과 기록** (커밋 d1ce189)

`docs/autopilot/ua-refresh/HANDOFF.md` 8절 "환경 사실" 목록 끝에 한 줄 추가: `- launchd 인증 실험(2026-09-04): Keychain 으로 됨 / 안 되어 oauth_token 사용 중` (해당하는 쪽). 커밋:

```bash
git add docs/autopilot/ua-refresh/HANDOFF.md
git commit -m "docs: record launchd auth probe result"
```

- [x] **Step 5: `/demiurge:rl` 로 완료조건 검증** (3개 기준 통과)

---

### Task 2: 모듈 초기화 · 루트 커맨드 · justfile

**Files:**
- Create: `go.mod`, `main.go`, `cmd/root.go`, `cmd/root_test.go`, `cmd/uarefresh/uarefresh.go`, `justfile`, `.gitignore`

**Interfaces:**
- Consumes: 없음
- Produces:
  - `cmd.Root() *cobra.Command` — 루트 커맨드. 도구는 `root.AddCommand(uarefresh.Cmd())` 한 줄로 붙는다.
  - `uarefresh.Cmd() *cobra.Command` (패키지 `gofer/cmd/uarefresh`) — `Use: "ua-refresh"`. 이후 Task 들이 이 함수 안에 `cmd.AddCommand(...)` 를 한 줄씩 추가한다.
  - `main.version` 변수 — `just build` 가 `-ldflags "-X main.version=..."` 로 채운다.

**스킬**: `superpowers:test-driven-development`.

**완료조건**: `go build -o bin/gofer . && ./bin/gofer --help` 출력에 `ua-refresh` 가 있고, `./bin/gofer ua-refresh --help` 가 종료 코드 0. `go test ./... && go vet ./...` 통과. 직접 의존성은 이 시점에 cobra·fang 2개(`go list -m -f '{{if not .Indirect}}{{.Path}}{{end}}' all`).

- [x] **Step 1: 모듈과 의존성**

```bash
go mod init gofer
go get github.com/spf13/cobra@v1.10.2 github.com/charmbracelet/fang@v1.0.0
```

- [x] **Step 2: 실패하는 테스트 — 루트 help 에 도구가 보인다**

`cmd/root_test.go`:

```go
package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootHelpListsTools(t *testing.T) {
	root := Root()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.String(), "ua-refresh") {
		t.Fatalf("help should list ua-refresh, got:\n%s", out.String())
	}
}
```

- [x] **Step 3: 실패 확인**

Run: `go test ./cmd/`
Expected: 컴파일 실패 (`undefined: Root`).

- [x] **Step 4: 구현**

`cmd/uarefresh/uarefresh.go`:

```go
// Package uarefresh 는 `gofer ua-refresh` 서브커맨드 트리다. 플래그 파싱만 하고
// 로직은 internal/uarefresh 에 둔다.
package uarefresh

import "github.com/spf13/cobra"

// Cmd 는 `ua-refresh` 부모 커맨드를 만든다. 서브커맨드는 각 파일에서 하나씩 붙인다.
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ua-refresh",
		Short: "Refresh Understand-Anything knowledge graphs of your repos every morning",
	}
	return cmd
}
```

`cmd/root.go`:

```go
// Package cmd 는 gofer 루트 커맨드다. 도구는 여기에 한 줄씩 등록한다.
package cmd

import (
	"github.com/spf13/cobra"

	"gofer/cmd/uarefresh"
)

// Root 는 `gofer` 루트 커맨드를 만든다.
func Root() *cobra.Command {
	root := &cobra.Command{
		Use:   "gofer",
		Short: "Personal CLI errand runner: gofer <tool> <action>",
	}
	root.AddCommand(uarefresh.Cmd())
	return root
}
```

`main.go`:

```go
package main

import (
	"context"
	"os"

	"github.com/charmbracelet/fang"

	"gofer/cmd"
)

// version 은 `just build` 가 -ldflags 로 채운다.
var version = "dev"

func main() {
	if err := fang.Execute(context.Background(), cmd.Root(), fang.WithVersion(version)); err != nil {
		os.Exit(1)
	}
}
```

`justfile`:

```just
version := `git describe --tags --always --dirty 2>/dev/null || echo dev`

# 빌드 결과는 bin/gofer
build:
    go build -ldflags "-X main.version={{version}}" -o bin/gofer .

test:
    go test ./...

lint:
    go vet ./...
```

`.gitignore`:

```
bin/
```

- [x] **Step 5: 통과 확인**

Run: `go mod tidy && go test ./... && go vet ./... && just build && ./bin/gofer --help && ./bin/gofer ua-refresh --help`
Expected: 테스트 PASS, help 에 `ua-refresh` 표시.

- [x] **Step 6: 커밋**

```bash
git add go.mod go.sum main.go cmd/ justfile .gitignore
git commit -m "feat: scaffold gofer binary with ua-refresh command tree"
```

- [x] **Step 7: `/demiurge:rl` 로 완료조건 검증**

---

### Task 3: 설정 파일 — 타입 · 로드 · 검증 · 템플릿 · `config init`

**Files:**
- Create: `internal/uarefresh/paths.go`, `internal/uarefresh/config.go`, `internal/uarefresh/config_test.go`, `cmd/uarefresh/config.go`
- Modify: `cmd/uarefresh/uarefresh.go` (서브커맨드 등록 한 줄)

**Interfaces:**
- Consumes: Task 2 의 `uarefresh.Cmd()`.
- Produces (패키지 `gofer/internal/uarefresh`):

```go
type Paths struct{ Config, LogDir, StateDir, Plist string }
func DefaultPaths() (Paths, error)                 // os.UserHomeDir 기준 (설계 3절 표)
func (p Paths) DailyLog(t time.Time) string        // LogDir/YYYY-MM-DD.log
func (p Paths) LaunchdLog() string                 // LogDir/launchd.log
func (p Paths) Lock() string                       // StateDir/run.lock
func (p Paths) LastRun() string                    // StateDir/last-run.json

type Config struct {
	Schedule ScheduleConfig `toml:"schedule"`
	Claude   ClaudeConfig   `toml:"claude"`
	Notify   NotifyConfig   `toml:"notify"`
	Env      EnvConfig      `toml:"env"`
	Repos    []RepoConfig   `toml:"repos"`
}
type ScheduleConfig struct{ At string `toml:"at"` }
type ClaudeConfig struct {
	BudgetUSD  float64 `toml:"budget_usd"`
	TimeoutMin int     `toml:"timeout_min"`
	Model      string  `toml:"model"`
	OAuthToken string  `toml:"oauth_token"`
}
type NotifyConfig struct{ Slack SlackConfig `toml:"slack"` }
type SlackConfig struct{ WebhookURL string `toml:"webhook_url"` }
type EnvConfig struct{ ExtraPath []string `toml:"extra_path"` }
type RepoConfig struct {
	Path  string `toml:"path"`   // Load 후에는 ~ 가 풀린 절대 경로
	Trunk string `toml:"trunk"`
}
func (r RepoConfig) Name() string                  // filepath.Base(r.Path)
func (c ClaudeConfig) Timeout() time.Duration      // TimeoutMin 분

const Template string                               // 설계 3절의 TOML 템플릿 원문
func Load(path string) (*Config, error)            // 디코드 + ~ 확장 + Validate. 오류는 errors.Join 으로 전부 모아 반환
func Validate(c *Config) []error
func WriteTemplate(path string) error              // 이미 있으면 os.ErrExist 로 실패. 디렉토리 0700, 파일 0600
func ExpandHome(p string) string                   // "~/x" → "$HOME/x"
func ResolveExtraPath(entries []string) []string   // 항목마다 glob → 이름순 정렬 → 마지막. 매치 없으면 제외
```

**스킬**: `superpowers:test-driven-development`.

**완료조건**: `go test ./internal/uarefresh/ -run 'Config|Paths|ExtraPath|Template'` PASS. `HOME=$(mktemp -d) ./bin/gofer ua-refresh config init` 이 `$HOME/.config/gofer/ua-refresh.toml` 을 0600 으로 만들고, 같은 명령을 한 번 더 실행하면 "already exists" 오류로 종료 코드 1 이며 파일 내용이 그대로다.

- [x] **Step 1: 실패하는 테스트**

`internal/uarefresh/config_test.go`:

```go
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
```

- [x] **Step 2: 실패 확인**

Run: `go test ./internal/uarefresh/`
Expected: 컴파일 실패 (`undefined: Load` 등).

- [x] **Step 3: 구현**

```bash
go get github.com/BurntSushi/toml@v1.6.0
```

`internal/uarefresh/paths.go`:

```go
// Package uarefresh 는 ua-refresh 도구의 로직이다. cmd/ 는 이 패키지를 호출만 한다.
package uarefresh

import (
	"os"
	"path/filepath"
	"time"
)

// Paths 는 설계 3절 "파일 위치" 표다. 테스트는 임시 디렉토리로 직접 만든다.
type Paths struct {
	Config   string // ~/.config/gofer/ua-refresh.toml
	LogDir   string // ~/Library/Logs/gofer/ua-refresh
	StateDir string // ~/Library/Application Support/gofer/ua-refresh
	Plist    string // ~/Library/LaunchAgents/gofer.ua-refresh.plist
}

// LaunchdLabel 은 launchd 작업 이름이다.
const LaunchdLabel = "gofer.ua-refresh"

// DefaultPaths 는 홈 디렉토리 기준 기본 위치를 만든다.
func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	return Paths{
		Config:   filepath.Join(home, ".config", "gofer", "ua-refresh.toml"),
		LogDir:   filepath.Join(home, "Library", "Logs", "gofer", "ua-refresh"),
		StateDir: filepath.Join(home, "Library", "Application Support", "gofer", "ua-refresh"),
		Plist:    filepath.Join(home, "Library", "LaunchAgents", LaunchdLabel+".plist"),
	}, nil
}

func (p Paths) DailyLog(t time.Time) string { return filepath.Join(p.LogDir, t.Format("2006-01-02")+".log") }
func (p Paths) LaunchdLog() string          { return filepath.Join(p.LogDir, "launchd.log") }
func (p Paths) Lock() string                { return filepath.Join(p.StateDir, "run.lock") }
func (p Paths) LastRun() string             { return filepath.Join(p.StateDir, "last-run.json") }
```

`internal/uarefresh/config.go`:

```go
package uarefresh

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Config 는 ~/.config/gofer/ua-refresh.toml 전체다 (설계 3절).
type Config struct {
	Schedule ScheduleConfig `toml:"schedule"`
	Claude   ClaudeConfig   `toml:"claude"`
	Notify   NotifyConfig   `toml:"notify"`
	Env      EnvConfig      `toml:"env"`
	Repos    []RepoConfig   `toml:"repos"`
}

type ScheduleConfig struct {
	At string `toml:"at"` // "HH:MM"
}

type ClaudeConfig struct {
	BudgetUSD  float64 `toml:"budget_usd"`
	TimeoutMin int     `toml:"timeout_min"`
	Model      string  `toml:"model"`
	OAuthToken string  `toml:"oauth_token"`
}

// Timeout 은 레포당 시간 상한이다.
func (c ClaudeConfig) Timeout() time.Duration { return time.Duration(c.TimeoutMin) * time.Minute }

type NotifyConfig struct {
	Slack SlackConfig `toml:"slack"`
}

type SlackConfig struct {
	WebhookURL string `toml:"webhook_url"`
}

type EnvConfig struct {
	ExtraPath []string `toml:"extra_path"`
}

type RepoConfig struct {
	Path  string `toml:"path"` // Load 후에는 ~ 가 풀린 경로
	Trunk string `toml:"trunk"`
}

// Name 은 --only, 화면, DM 에 쓰는 레포 이름(마지막 디렉토리명)이다.
func (r RepoConfig) Name() string { return filepath.Base(r.Path) }

// Template 은 `config init` 이 쓰는 설정 템플릿이다 (설계 3절 원문).
const Template = `[schedule]
at = "07:30"                 # install 이 plist 에 반영

[claude]
budget_usd  = 20             # 레포당 지출 상한 (폭주 방지용)
timeout_min = 60             # 레포당 시간 상한
model       = ""             # 비우면 Claude Code 기본 설정
oauth_token = ""             # launchd 에서 Keychain 인증이 안 될 때만

[notify.slack]
webhook_url = "https://hooks.slack.com/services/..."

[env]
# launchd 의 빈 PATH 를 보완. glob 은 이름순 정렬 후 마지막(최신) 항목을 고른다.
extra_path = ["~/.local/bin", "~/.nvm/versions/node/*/bin", "/opt/homebrew/bin"]

[[repos]]
path  = "~/src/example-api"
trunk = "main"

[[repos]]
path  = "~/src/example-web"
trunk = "develop"
`

// Load 는 설정을 읽고 ~ 를 풀고 검증한다. 오류는 전부 모아 하나로 돌려준다 (설계 3·7절).
func Load(path string) (*Config, error) {
	var cfg Config
	md, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var errs []error
	for _, k := range md.Undecoded() {
		errs = append(errs, fmt.Errorf("unknown key %q", k.String()))
	}
	for i := range cfg.Repos {
		cfg.Repos[i].Path = ExpandHome(cfg.Repos[i].Path)
	}
	for i := range cfg.Env.ExtraPath {
		cfg.Env.ExtraPath[i] = ExpandHome(cfg.Env.ExtraPath[i])
	}
	errs = append(errs, Validate(&cfg)...)
	if len(errs) > 0 {
		return nil, fmt.Errorf("config %s:\n%w", path, errors.Join(errs...))
	}
	return &cfg, nil
}

// Validate 는 필수 항목과 형식을 검사해 문제를 전부 돌려준다.
func Validate(c *Config) []error {
	var errs []error
	if _, err := time.Parse("15:04", c.Schedule.At); err != nil {
		errs = append(errs, fmt.Errorf("schedule.at must be HH:MM, got %q", c.Schedule.At))
	}
	if c.Claude.BudgetUSD <= 0 {
		errs = append(errs, errors.New("claude.budget_usd must be > 0"))
	}
	if c.Claude.TimeoutMin <= 0 {
		errs = append(errs, errors.New("claude.timeout_min must be > 0"))
	}
	if u := c.Notify.Slack.WebhookURL; !strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "http://") {
		errs = append(errs, errors.New("notify.slack.webhook_url is required (http(s) URL)"))
	}
	if len(c.Repos) == 0 {
		errs = append(errs, errors.New("at least one [[repos]] entry is required"))
	}
	for i, r := range c.Repos {
		if st, err := os.Stat(filepath.Join(r.Path, ".git")); err != nil || !st.IsDir() {
			errs = append(errs, fmt.Errorf("repos[%d].path %q is not a git repository root", i, r.Path))
		}
		if r.Trunk == "" {
			errs = append(errs, fmt.Errorf("repos[%d].trunk is required", i))
		}
	}
	return errs
}

// WriteTemplate 은 템플릿을 0600 으로 새로 만든다. 이미 있으면 os.ErrExist 를 돌려주고 건드리지 않는다.
func WriteTemplate(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(Template)
	return err
}

// ExpandHome 은 앞의 "~" 를 홈 디렉토리로 바꾼다.
func ExpandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

// ResolveExtraPath 는 각 항목을 glob 으로 풀어 이름순 마지막 하나를 고른다. 매치가 없으면 뺀다.
func ResolveExtraPath(entries []string) []string {
	var out []string
	for _, e := range entries {
		matches, _ := filepath.Glob(e)
		if len(matches) == 0 {
			continue
		}
		sort.Strings(matches)
		out = append(out, matches[len(matches)-1])
	}
	return out
}
```

`cmd/uarefresh/config.go`:

```go
package uarefresh

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	ua "gofer/internal/uarefresh"
)

func configCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Manage the ua-refresh config file"}
	cmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Write a config template to ~/.config/gofer/ua-refresh.toml (never overwrites)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			paths, err := ua.DefaultPaths()
			if err != nil {
				return err
			}
			if err := ua.WriteTemplate(paths.Config); err != nil {
				if errors.Is(err, os.ErrExist) {
					return fmt.Errorf("%s already exists; edit it directly", paths.Config)
				}
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %s — edit repos and webhook_url\n", paths.Config)
			return nil
		},
	})
	return cmd
}
```

`cmd/uarefresh/uarefresh.go` 의 `Cmd()` 안, `return cmd` 앞에 추가:

```go
	cmd.AddCommand(configCmd())
```

- [x] **Step 4: 통과 확인**

Run: `go test ./internal/uarefresh/ ./cmd/... && go vet ./... && just build && HOME=$(mktemp -d) sh -c './bin/gofer ua-refresh config init && ls -l $HOME/.config/gofer/ && ./bin/gofer ua-refresh config init; echo exit=$?'`
Expected: PASS; 첫 init 은 `wrote ...`, 파일 권한 `-rw-------`, 두 번째는 `already exists` 오류에 `exit=1`.

- [x] **Step 5: 커밋**

```bash
git add internal/uarefresh/paths.go internal/uarefresh/config.go internal/uarefresh/config_test.go cmd/uarefresh/ go.mod go.sum
git commit -m "feat(ua-refresh): config schema, validation and config init"
```

- [x] **Step 6: `/demiurge:rl` 로 완료조건 검증**

---

### Task 4: git 래퍼

**Files:**
- Create: `internal/uarefresh/git.go`, `internal/uarefresh/git_test.go`, `internal/uarefresh/gittest_test.go`(테스트 헬퍼, 이후 Task 들이 재사용)

**Interfaces:**
- Consumes: 없음
- Produces (패키지 `gofer/internal/uarefresh`):

```go
func CurrentBranch(ctx context.Context, dir string) (string, error)          // detached 면 ""
func HasTrackedChanges(ctx context.Context, dir string) (bool, error)         // git status --porcelain --untracked-files=no
func Fetch(ctx context.Context, dir string) (logText string, err error)       // git fetch -ptf, stderr 를 로그용으로 반환
func Head(ctx context.Context, dir string) (string, error)                   // git rev-parse HEAD
func CountCommits(ctx context.Context, dir, from, to string) (int, error)    // git rev-list --count from..to
func FFMerge(ctx context.Context, dir, trunk string) error                    // git merge --ff-only origin/<trunk>
func CommitExists(ctx context.Context, dir, hash string) bool                 // git cat-file -e <hash>^{commit}
```

테스트 헬퍼 (같은 패키지, `_test.go`):

```go
func isolateGit(t *testing.T)                                               // 전역 git 설정 격리 + user.name/email
func run(t *testing.T, dir, name string, args ...string) string             // 외부 명령 실행, 실패 시 t.Fatal
func newRepoWithOrigin(t *testing.T, trunk string) (work, origin string)    // bare origin + 작업 트리(첫 커밋 push 됨)
func commitFile(t *testing.T, dir, name, content string) (hash string)      // 파일 추가 커밋
func pushFromClone(t *testing.T, origin, trunk, name, content string) (hash string) // 다른 clone 에서 커밋해 origin 을 앞서게 함
```

**스킬**: `superpowers:test-driven-development`.

**완료조건**: `go test ./internal/uarefresh/ -run 'Git|Branch|Tracked|Fetch|FFMerge|CommitExists'` PASS. 사용자 전역 git 설정(`commit.gpgsign` 등)과 무관하게 통과한다 — 검증: `GIT_CONFIG_GLOBAL=/dev/null go test ./internal/uarefresh/` 도 PASS.

- [x] **Step 1: 테스트 헬퍼**

`internal/uarefresh/gittest_test.go`:

```go
package uarefresh

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// isolateGit 은 사용자 전역 git 설정(서명 등)이 테스트에 끼어들지 않게 한다.
func isolateGit(t *testing.T) {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	body := "[user]\n\tname = test\n\temail = test@example.com\n[commit]\n\tgpgsign = false\n[init]\n\tdefaultBranch = main\n"
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

// run 은 외부 명령을 dir 에서 실행하고 출력을 돌려준다. 실패하면 테스트를 멈춘다.
func run(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// newRepoWithOrigin 은 bare origin 과 그것을 추적하는 작업 트리 example-api 를 만든다. 첫 커밋이 push 돼 있다.
func newRepoWithOrigin(t *testing.T, trunk string) (work, origin string) {
	t.Helper()
	isolateGit(t)
	base := t.TempDir()
	origin = filepath.Join(base, "origin.git")
	work = filepath.Join(base, "example-api")
	run(t, base, "git", "init", "-q", "--bare", "-b", trunk, origin)
	run(t, base, "git", "init", "-q", "-b", trunk, work)
	run(t, work, "git", "remote", "add", "origin", origin)
	commitFile(t, work, "README.md", "hello\n")
	run(t, work, "git", "push", "-q", "-u", "origin", trunk)
	return work, origin
}

// commitFile 은 파일을 쓰고 커밋한 뒤 새 HEAD 해시를 돌려준다.
func commitFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "git", "add", name)
	run(t, dir, "git", "commit", "-q", "-m", "add "+name)
	return run(t, dir, "git", "rev-parse", "HEAD")
}

// pushFromClone 은 별도 clone 에서 커밋해 origin 을 앞서게 만든다. 돌려주는 값은 origin 의 새 HEAD.
func pushFromClone(t *testing.T, origin, trunk, name, content string) string {
	t.Helper()
	other := filepath.Join(t.TempDir(), "other")
	run(t, filepath.Dir(other), "git", "clone", "-q", "-b", trunk, origin, other)
	hash := commitFile(t, other, name, content)
	run(t, other, "git", "push", "-q", "origin", trunk)
	return hash
}
```

- [x] **Step 2: 실패하는 테스트**

`internal/uarefresh/git_test.go`:

```go
package uarefresh

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCurrentBranchAndDetached(t *testing.T) {
	work, _ := newRepoWithOrigin(t, "main")
	ctx := t.Context()
	if b, err := CurrentBranch(ctx, work); err != nil || b != "main" {
		t.Fatalf("branch = %q, %v", b, err)
	}
	run(t, work, "git", "checkout", "-q", "--detach")
	if b, err := CurrentBranch(ctx, work); err != nil || b != "" {
		t.Fatalf("detached branch = %q, %v", b, err)
	}
}

func TestHasTrackedChangesIgnoresUntracked(t *testing.T) {
	work, _ := newRepoWithOrigin(t, "main")
	ctx := t.Context()
	if dirty, _ := HasTrackedChanges(ctx, work); dirty {
		t.Fatal("clean repo reported dirty")
	}
	os.WriteFile(filepath.Join(work, "scratch.txt"), []byte("x"), 0o644)
	if dirty, _ := HasTrackedChanges(ctx, work); dirty {
		t.Fatal("untracked file should not count")
	}
	os.WriteFile(filepath.Join(work, "README.md"), []byte("changed"), 0o644)
	if dirty, _ := HasTrackedChanges(ctx, work); !dirty {
		t.Fatal("modified tracked file should count")
	}
}

func TestFetchCountAndFFMerge(t *testing.T) {
	work, origin := newRepoWithOrigin(t, "main")
	ctx := t.Context()
	pushFromClone(t, origin, "main", "a.txt", "a")
	want := pushFromClone(t, origin, "main", "b.txt", "b")
	if _, err := Fetch(ctx, work); err != nil {
		t.Fatal(err)
	}
	if n, err := CountCommits(ctx, work, "HEAD", "origin/main"); err != nil || n != 2 {
		t.Fatalf("behind = %d, %v", n, err)
	}
	if err := FFMerge(ctx, work, "main"); err != nil {
		t.Fatal(err)
	}
	if h, _ := Head(ctx, work); h != want {
		t.Fatalf("HEAD = %s want %s", h, want)
	}
}

func TestFFMergeFailsWhenLocalAhead(t *testing.T) {
	work, origin := newRepoWithOrigin(t, "main")
	ctx := t.Context()
	commitFile(t, work, "local.txt", "l")
	pushFromClone(t, origin, "main", "remote.txt", "r")
	if _, err := Fetch(ctx, work); err != nil {
		t.Fatal(err)
	}
	if err := FFMerge(ctx, work, "main"); err == nil {
		t.Fatal("diverged ff-merge should fail")
	}
	if n, _ := CountCommits(ctx, work, "origin/main", "HEAD"); n != 1 {
		t.Fatalf("ahead = %d", n)
	}
}

func TestCommitExists(t *testing.T) {
	work, _ := newRepoWithOrigin(t, "main")
	ctx := t.Context()
	h, _ := Head(ctx, work)
	if !CommitExists(ctx, work, h) {
		t.Fatal("HEAD should exist")
	}
	if CommitExists(ctx, work, "0000000000000000000000000000000000000000") {
		t.Fatal("zero hash should not exist")
	}
}
```

- [x] **Step 3: 실패 확인**

Run: `go test ./internal/uarefresh/ -run 'Branch|Tracked|Fetch|FFMerge|CommitExists'`
Expected: 컴파일 실패 (`undefined: CurrentBranch` 등).

- [x] **Step 4: 구현**

`internal/uarefresh/git.go`:

```go
package uarefresh

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// git 은 `git -C dir args...` 를 실행한다. 실패하면 stderr 를 오류 문구에 담는다.
func git(ctx context.Context, dir string, args ...string) (stdout, stderr string, err error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err = cmd.Run()
	stdout, stderr = strings.TrimSpace(out.String()), strings.TrimSpace(errb.String())
	if err != nil {
		msg := stderr
		if msg == "" {
			msg = err.Error()
		}
		err = fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return stdout, stderr, err
}

// CurrentBranch 는 현재 브랜치 이름이다. detached HEAD 면 "" 를 돌려준다.
func CurrentBranch(ctx context.Context, dir string) (string, error) {
	out, _, err := git(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	if out == "HEAD" {
		return "", nil
	}
	return out, nil
}

// HasTrackedChanges 는 추적 파일에 미커밋 수정이 있는지 본다. 미추적 파일은 무시한다 (설계 4절).
func HasTrackedChanges(ctx context.Context, dir string) (bool, error) {
	out, _, err := git(ctx, dir, "status", "--porcelain", "--untracked-files=no")
	return out != "", err
}

// Fetch 는 `git fetch -ptf` 다. git 이 stderr 로 내는 진행 요약을 로그용으로 돌려준다.
func Fetch(ctx context.Context, dir string) (string, error) {
	_, stderr, err := git(ctx, dir, "fetch", "-ptf")
	return stderr, err
}

// Head 는 현재 HEAD 커밋 해시다.
func Head(ctx context.Context, dir string) (string, error) {
	out, _, err := git(ctx, dir, "rev-parse", "HEAD")
	return out, err
}

// CountCommits 는 from..to 사이 커밋 수다. 예: CountCommits(dir, "HEAD", "origin/main") = 뒤처진 수.
func CountCommits(ctx context.Context, dir, from, to string) (int, error) {
	out, _, err := git(ctx, dir, "rev-list", "--count", from+".."+to)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(out)
}

// FFMerge 는 origin/<trunk> 를 fast-forward 로만 합친다. 분기돼 있으면 실패한다.
func FFMerge(ctx context.Context, dir, trunk string) error {
	_, _, err := git(ctx, dir, "merge", "--ff-only", "origin/"+trunk)
	return err
}

// CommitExists 는 해시가 이 레포에 커밋으로 존재하는지 본다 (force-push 뒤엔 사라질 수 있다).
func CommitExists(ctx context.Context, dir, hash string) bool {
	_, _, err := git(ctx, dir, "cat-file", "-e", hash+"^{commit}")
	return err == nil
}
```

- [x] **Step 5: 통과 확인**

Run: `go test ./internal/uarefresh/ && GIT_CONFIG_GLOBAL=/dev/null go test ./internal/uarefresh/ && go vet ./...`
Expected: PASS 두 번.

- [x] **Step 6: 커밋**

```bash
git add internal/uarefresh/git.go internal/uarefresh/git_test.go internal/uarefresh/gittest_test.go
git commit -m "feat(ua-refresh): git wrapper with temp-repo tests"
```

- [x] **Step 7: `/demiurge:rl` 로 완료조건 검증**

---

### Task 5: 그래프 메타 — 데이터 디렉토리 · 해시 · 결정

**Files:**
- Create: `internal/uarefresh/graph.go`, `internal/uarefresh/graph_test.go`

**Interfaces:**
- Consumes: Task 4 의 `CommitExists`, 헬퍼 `newRepoWithOrigin`, `commitFile`.
- Produces:

```go
func GraphDir(repo string) string                       // <repo>/.understand-anything 이 있으면 그것, 아니면 <repo>/.ua
var ErrNoGraph = errors.New("no knowledge graph")
func GraphHash(repo string) (string, error)             // meta.json 의 gitCommitHash. 없으면 ErrNoGraph

type GraphDecision int
const (
	GraphUpToDate GraphDecision = iota // 해시 == HEAD → Claude 호출 안 함
	GraphRefresh                       // 해시 != HEAD (해시가 레포에 있음) 또는 그래프 없음 → /understand
	GraphFull                          // 해시가 레포에 없음 → /understand --full
)
func (d GraphDecision) String() string                  // "up to date" / "/understand" / "/understand --full"
func DecideGraph(ctx context.Context, repo, head string) (d GraphDecision, graphHash string, err error)
```

**스킬**: `superpowers:test-driven-development`.

**완료조건**: `go test ./internal/uarefresh/ -run Graph` PASS — 다섯 경우(그래프 없음 / 해시==HEAD / 해시 다르고 존재 / 해시 없는 커밋 / 레거시 디렉토리 우선) 모두.

- [x] **Step 1: 실패하는 테스트**

`internal/uarefresh/graph_test.go`:

```go
package uarefresh

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeMeta(t *testing.T, repo, dir, hash string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repo, dir), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"version":"2.9.4","gitCommitHash":"` + hash + `"}`
	if err := os.WriteFile(filepath.Join(repo, dir, "meta.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGraphDirPrefersLegacy(t *testing.T) {
	repo := t.TempDir()
	if got := GraphDir(repo); got != filepath.Join(repo, ".ua") {
		t.Errorf("default = %q", got)
	}
	os.MkdirAll(filepath.Join(repo, ".understand-anything"), 0o755)
	if got := GraphDir(repo); got != filepath.Join(repo, ".understand-anything") {
		t.Errorf("legacy = %q", got)
	}
}

func TestGraphHashMissing(t *testing.T) {
	_, err := GraphHash(t.TempDir())
	if !errors.Is(err, ErrNoGraph) {
		t.Fatalf("want ErrNoGraph, got %v", err)
	}
}

func TestDecideGraph(t *testing.T) {
	work, _ := newRepoWithOrigin(t, "main")
	ctx := t.Context()
	first, _ := Head(ctx, work)
	second := commitFile(t, work, "x.txt", "x")

	cases := []struct {
		name     string
		setup    func()
		want     GraphDecision
		wantHash string
	}{
		{"no graph", func() {}, GraphRefresh, ""},
		{"hash equals head", func() { writeMeta(t, work, ".ua", second) }, GraphUpToDate, second},
		{"hash behind head", func() { writeMeta(t, work, ".ua", first) }, GraphRefresh, first},
		{"hash not in repo", func() { writeMeta(t, work, ".ua", "0123456789abcdef0123456789abcdef01234567") }, GraphFull, "0123456789abcdef0123456789abcdef01234567"},
		{"legacy dir wins", func() { writeMeta(t, work, ".ua", first); writeMeta(t, work, ".understand-anything", second) }, GraphUpToDate, second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			os.RemoveAll(filepath.Join(work, ".ua"))
			os.RemoveAll(filepath.Join(work, ".understand-anything"))
			tc.setup()
			d, h, err := DecideGraph(ctx, work, second)
			if err != nil || d != tc.want || h != tc.wantHash {
				t.Fatalf("got (%v, %q, %v) want (%v, %q)", d, h, err, tc.want, tc.wantHash)
			}
		})
	}
}
```

- [x] **Step 2: 실패 확인**

Run: `go test ./internal/uarefresh/ -run Graph`
Expected: 컴파일 실패 (`undefined: GraphDir`).

- [x] **Step 3: 구현**

`internal/uarefresh/graph.go`:

```go
package uarefresh

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNoGraph 는 그래프 디렉토리나 meta.json 이 없을 때다.
var ErrNoGraph = errors.New("no knowledge graph")

// GraphDir 는 /understand 의 데이터 디렉토리다. 레거시 .understand-anything/ 이 있으면 그것, 아니면 .ua/ (설계 2절).
func GraphDir(repo string) string {
	legacy := filepath.Join(repo, ".understand-anything")
	if st, err := os.Stat(legacy); err == nil && st.IsDir() {
		return legacy
	}
	return filepath.Join(repo, ".ua")
}

// GraphHash 는 meta.json 에 기록된 마지막 분석 커밋이다.
func GraphHash(repo string) (string, error) {
	b, err := os.ReadFile(filepath.Join(GraphDir(repo), "meta.json"))
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrNoGraph
	}
	if err != nil {
		return "", err
	}
	var meta struct {
		GitCommitHash string `json:"gitCommitHash"`
	}
	if err := json.Unmarshal(b, &meta); err != nil {
		return "", fmt.Errorf("parse meta.json: %w", err)
	}
	if meta.GitCommitHash == "" {
		return "", ErrNoGraph
	}
	return meta.GitCommitHash, nil
}

// GraphDecision 은 설계 4절 ④ 의 세 갈래다.
type GraphDecision int

const (
	GraphUpToDate GraphDecision = iota // 해시 == HEAD: Claude 호출 안 함
	GraphRefresh                       // 증분 (또는 그래프가 아예 없어 스킬이 알아서 전체 분석)
	GraphFull                          // 해시가 레포에 없음: --full
)

func (d GraphDecision) String() string {
	switch d {
	case GraphUpToDate:
		return "up to date"
	case GraphFull:
		return "/understand --full"
	default:
		return "/understand"
	}
}

// DecideGraph 는 그래프 해시와 HEAD 를 비교해 무엇을 할지 정한다. /understand 는 해시가 같으면 사용자에게
// 되묻고 멈추므로 무인 실행에서는 여기서 먼저 걸러야 한다 (설계 2·9절).
func DecideGraph(ctx context.Context, repo, head string) (GraphDecision, string, error) {
	hash, err := GraphHash(repo)
	if errors.Is(err, ErrNoGraph) {
		return GraphRefresh, "", nil
	}
	if err != nil {
		return 0, "", err
	}
	switch {
	case hash == head:
		return GraphUpToDate, hash, nil
	case !CommitExists(ctx, repo, hash):
		return GraphFull, hash, nil
	default:
		return GraphRefresh, hash, nil
	}
}
```

- [x] **Step 4: 통과 확인**

Run: `go test ./internal/uarefresh/ && go vet ./...`
Expected: PASS.

- [x] **Step 5: 커밋**

```bash
git add internal/uarefresh/graph.go internal/uarefresh/graph_test.go
git commit -m "feat(ua-refresh): graph metadata and refresh decision"
```

- [x] **Step 6: `/demiurge:rl` 로 완료조건 검증**

---

### Task 6: Claude 실행 — PATH 조립 · 프로세스 그룹 타임아웃 · JSON 파싱

**Files:**
- Create: `internal/uarefresh/claude.go`, `internal/uarefresh/claude_test.go`, `internal/uarefresh/fakeclaude_test.go`(헬퍼, Task 9·12 재사용)

**Interfaces:**
- Consumes: 없음 (Task 4 헬퍼 `newRepoWithOrigin` 은 테스트에서만)
- Produces:

```go
type ClaudeOptions struct {
	Dir        string        // cwd = 레포 루트
	Full       bool          // "/understand --full"
	BudgetUSD  float64       // --max-budget-usd
	Timeout    time.Duration // 레포당 시간 상한
	Model      string        // 비면 --model 생략
	ExtraPath  []string      // PATH 앞에 붙일 디렉토리 (ResolveExtraPath 결과)
	OAuthToken string        // 있으면 CLAUDE_CODE_OAUTH_TOKEN
	Stderr     io.Writer     // claude 의 stderr 를 보낼 곳 (nil 이면 버림)
}
type ClaudeResult struct {
	IsError      bool    `json:"is_error"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Result       string  `json:"result"`
	TimedOut     bool    `json:"-"`
}
func ClaudeArgs(o ClaudeOptions) []string                                   // 실제 인자 (dry-run 표시에도 사용)
func RunClaude(ctx context.Context, o ClaudeOptions) (ClaudeResult, error)   // 오류여도 파싱된 비용은 Result 에 남는다
```

테스트 헬퍼:

```go
type fakeClaude struct{ Dir, Args, Env, Cwd, Child string }
func installFakeClaude(t *testing.T, mode string) fakeClaude   // mode: ok | noop | error | hang | garbage
func (f fakeClaude) called() bool
func (f fakeClaude) argsLines(t *testing.T) []string
```

**스킬**: `superpowers:test-driven-development`.

**완료조건**: `go test ./internal/uarefresh/ -run Claude` PASS — 인자·환경·cwd 기록 일치, `is_error` 오류, JSON 아님 오류, 타임아웃 시 손자 프로세스(`sleep`)까지 죽음, 프로세스 PATH 가 비어도 `extra_path` 로 `claude` 를 찾음. 타임아웃 테스트는 3초 안에 끝난다.

- [x] **Step 1: 가짜 claude 헬퍼**

`internal/uarefresh/fakeclaude_test.go`:

```go
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
```

- [x] **Step 2: 실패하는 테스트**

`internal/uarefresh/claude_test.go`:

```go
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
```

- [x] **Step 3: 실패 확인**

Run: `go test ./internal/uarefresh/ -run Claude`
Expected: 컴파일 실패 (`undefined: RunClaude`).

- [x] **Step 4: 구현**

`internal/uarefresh/claude.go`:

```go
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
```

- [x] **Step 5: 통과 확인**

Run: `go test ./internal/uarefresh/ -run Claude -v 2>&1 | tail -20 && go vet ./...`
Expected: 6개 PASS, `TestRunClaudeTimeoutKillsProcessGroup` 이 3초 이내.

- [x] **Step 6: 커밋**

```bash
git add internal/uarefresh/claude.go internal/uarefresh/claude_test.go internal/uarefresh/fakeclaude_test.go
git commit -m "feat(ua-refresh): run claude with process-group timeout and json parsing"
```

- [x] **Step 7: `/demiurge:rl` 로 완료조건 검증**

---

### Task 7: 공용 화면 패키지 — Event 타입 · 집계 · 한 줄 로그

**Files:**
- Create: `internal/tui/event.go`, `internal/tui/tally.go`, `internal/tui/plain.go`, `internal/tui/plain_test.go`

**Interfaces:**
- Consumes: 없음
- Produces (패키지 `gofer/internal/tui`, 도구 공용 — ua-refresh 전용 문구 없음):

```go
type Kind int
const (
	KindStart   Kind = iota // 항목 처리 시작
	KindStage               // 단계 전환: Label = 단계명, Detail 선택 (예: "+13 commits")
	KindDone                // 항목 완료: Outcome·Label·Detail·Elapsed·CostUSD
	KindAllDone             // 전체 완료
)
type Outcome int
const (
	OutcomeOK      Outcome = iota // ✓ 했고 성공
	OutcomeNoop                   // – 할 일이 없었음
	OutcomeSkipped                // ↷ 조건이 안 맞아 건너뜀
	OutcomeFailed                 // ✗
)
func (o Outcome) Symbol() string
type Item struct{ Name, Sub string }                 // 한 줄에 보일 이름과 부제 (예: 레포명, trunk)
type Labels struct{ OK, Noop, Skipped, Failed string } // 요약 문구 (예: updated / up to date / skipped / failed)
type Event struct {
	Kind    Kind
	Index   int           // Items 의 인덱스
	Outcome Outcome       // KindDone
	Label   string        // 단계명 또는 결과 문구
	Detail  string        // 부가 정보
	Elapsed time.Duration // 0 이면 표시 안 함
	CostUSD float64       // 0 이면 표시 안 함
}
func Summary(l Labels, ok, noop, skipped, failed int, cost float64) string // "2 updated · 1 up to date · 0 skipped · 1 failed · $2.54"

type Plain struct {                // 한 줄 로그 소비자. TTY 가 아닐 때의 화면이자, TTY 일 때의 로그 파일 기록자
	W      io.Writer
	Items  []Item
	Labels Labels
	Now    func() time.Time
}
func (p *Plain) Write(ev Event)
```

**스킬**: `superpowers:test-driven-development`.

**완료조건**: `go test ./internal/tui/` PASS. 패키지에 ua-refresh 전용 단어("repo", "graph", "understand")가 없다 — 검증: `grep -n -i -E 'repo|graph|understand' internal/tui/*.go` 결과 없음.

- [x] **Step 1: 실패하는 테스트**

`internal/tui/plain_test.go`:

```go
package tui

import (
	"bytes"
	"testing"
	"time"
)

var testLabels = Labels{OK: "updated", Noop: "up to date", Skipped: "skipped", Failed: "failed"}

func fixedNow() time.Time { return time.Date(2026, 9, 5, 8, 41, 2, 0, time.UTC) }

func TestPlainWritesOneLinePerEvent(t *testing.T) {
	var buf bytes.Buffer
	p := &Plain{W: &buf, Items: []Item{{"example-api", "main"}, {"example-docs", "main"}}, Labels: testLabels, Now: fixedNow}
	for _, ev := range []Event{
		{Kind: KindStart, Index: 0},
		{Kind: KindStage, Index: 0, Label: "fetch"},
		{Kind: KindStage, Index: 0, Label: "/understand …", Detail: "+13 commits"},
		{Kind: KindDone, Index: 0, Outcome: OutcomeOK, Label: "graph updated", Detail: "+13 commits", Elapsed: 252 * time.Second, CostUSD: 1.83},
		{Kind: KindStart, Index: 1},
		{Kind: KindDone, Index: 1, Outcome: OutcomeNoop, Label: "up to date"},
		{Kind: KindAllDone},
	} {
		p.Write(ev)
	}
	want := `08:41:02 example-api start
08:41:02 example-api fetch
08:41:02 example-api /understand … +13 commits
08:41:02 example-api ✓ graph updated +13 commits 4m12s $1.83
08:41:02 example-docs start
08:41:02 example-docs – up to date
08:41:02 done · 1 updated · 1 up to date · 0 skipped · 0 failed · $1.83
`
	if buf.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", buf.String(), want)
	}
}

func TestSummary(t *testing.T) {
	got := Summary(testLabels, 2, 1, 0, 1, 2.54)
	want := "2 updated · 1 up to date · 0 skipped · 1 failed · $2.54"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestOutcomeSymbols(t *testing.T) {
	if OutcomeOK.Symbol()+OutcomeNoop.Symbol()+OutcomeSkipped.Symbol()+OutcomeFailed.Symbol() != "✓–↷✗" {
		t.Error("symbols changed")
	}
}
```

- [x] **Step 2: 실패 확인**

Run: `go test ./internal/tui/`
Expected: 컴파일 실패.

- [x] **Step 3: 구현**

`internal/tui/event.go`:

```go
// Package tui 는 "항목 N개를 순차 처리하며 상태를 보여주는" 도구 공용 화면이다. 처리기가 보내는 Event 만 소비하며,
// 터미널이면 bubbletea 화면을, 아니면 한 줄 로그를 만든다. 두 화면이 같은 Event 를 읽으므로 어긋날 수 없다.
package tui

import "time"

// Kind 는 Event 의 종류다.
type Kind int

const (
	KindStart   Kind = iota // 항목 처리 시작
	KindStage               // 단계 전환: Label 에 단계명, Detail 은 선택
	KindDone                // 항목 완료
	KindAllDone             // 전체 완료
)

// Outcome 은 항목 하나의 결과다.
type Outcome int

const (
	OutcomeOK      Outcome = iota // 했고 성공
	OutcomeNoop                   // 할 일이 없었음
	OutcomeSkipped                // 조건이 안 맞아 건너뜀
	OutcomeFailed                 // 실패
)

// Symbol 은 한 글자 표시다.
func (o Outcome) Symbol() string {
	switch o {
	case OutcomeOK:
		return "✓"
	case OutcomeNoop:
		return "–"
	case OutcomeSkipped:
		return "↷"
	default:
		return "✗"
	}
}

// Item 은 한 줄에 보일 항목이다.
type Item struct {
	Name string
	Sub  string
}

// Labels 는 요약 줄에 쓸 결과별 문구다.
type Labels struct {
	OK, Noop, Skipped, Failed string
}

// Event 는 처리기가 화면에 알리는 한 사건이다.
type Event struct {
	Kind    Kind
	Index   int
	Outcome Outcome
	Label   string
	Detail  string
	Elapsed time.Duration
	CostUSD float64
}
```

`internal/tui/tally.go`:

```go
package tui

import (
	"fmt"
	"strings"
	"time"
)

// Summary 는 요약 문구다. 예: "2 updated · 1 up to date · 0 skipped · 1 failed · $2.54".
func Summary(l Labels, ok, noop, skipped, failed int, cost float64) string {
	return fmt.Sprintf("%d %s · %d %s · %d %s · %d %s · $%.2f", ok, l.OK, noop, l.Noop, skipped, l.Skipped, failed, l.Failed, cost)
}

// tally 는 완료 Event 를 세어 요약 문구를 만든다.
type tally struct {
	counts [4]int
	cost   float64
}

func (t *tally) add(ev Event) {
	if ev.Kind == KindDone {
		t.counts[ev.Outcome]++
		t.cost += ev.CostUSD
	}
}

func (t tally) summary(l Labels) string {
	return Summary(l, t.counts[OutcomeOK], t.counts[OutcomeNoop], t.counts[OutcomeSkipped], t.counts[OutcomeFailed], t.cost)
}

func elapsedText(d time.Duration) string {
	if d <= 0 {
		return ""
	}
	return d.Round(time.Second).String()
}

func costText(c float64) string {
	if c <= 0 {
		return ""
	}
	return fmt.Sprintf("$%.2f", c)
}

// joinNonEmpty 는 빈 조각을 빼고 sep 로 잇는다.
func joinNonEmpty(sep string, parts ...string) string {
	var kept []string
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, sep)
}
```

`internal/tui/plain.go`:

```go
package tui

import (
	"fmt"
	"io"
	"time"
)

// Plain 은 Event 를 한 줄씩 쓴다. 터미널이 아닐 때(launchd)의 화면이고, 터미널일 때도 로그 파일에 같은 내용을 남긴다.
type Plain struct {
	W      io.Writer
	Items  []Item
	Labels Labels
	Now    func() time.Time
	tally  tally
}

// Write 는 Event 하나를 한 줄로 쓴다.
func (p *Plain) Write(ev Event) {
	p.tally.add(ev)
	ts := p.Now().Format("15:04:05")
	if ev.Kind == KindAllDone {
		fmt.Fprintf(p.W, "%s done · %s\n", ts, p.tally.summary(p.Labels))
		return
	}
	var body string
	switch ev.Kind {
	case KindStart:
		body = "start"
	case KindStage:
		body = joinNonEmpty(" ", ev.Label, ev.Detail)
	case KindDone:
		body = joinNonEmpty(" ", ev.Outcome.Symbol()+" "+ev.Label, ev.Detail, elapsedText(ev.Elapsed), costText(ev.CostUSD))
	}
	fmt.Fprintf(p.W, "%s %s %s\n", ts, p.Items[ev.Index].Name, body)
}
```

- [x] **Step 4: 통과 확인**

Run: `go test ./internal/tui/ && go vet ./... && grep -n -i -E 'repo|graph|understand' internal/tui/*.go; echo "grep exit=$?"`
Expected: PASS, grep 결과 없음(`grep exit=1`). (테스트 파일의 예시 문자열 "graph updated" 는 `_test.go` 라 검색 대상에 포함되지만 — `plain_test.go` 의 `/understand …`·`graph updated` 는 임의 문구다. 검사는 `internal/tui/*.go` 중 `_test.go` 를 제외하고 본다: `ls internal/tui/*.go | grep -v _test | xargs grep -n -i -E 'repo|graph|understand'`.)

- [x] **Step 5: 커밋**

```bash
git add internal/tui/
git commit -m "feat(tui): shared progress events, summary and plain-line consumer"
```

- [x] **Step 6: `/demiurge:rl` 로 완료조건 검증**

---

### Task 8: 결과 타입 · 잠금 파일 · Slack DM 본문

**Files:**
- Create: `internal/uarefresh/result.go`, `internal/uarefresh/result_test.go`, `internal/uarefresh/lock.go`, `internal/uarefresh/lock_test.go`, `internal/uarefresh/report.go`, `internal/uarefresh/report_test.go`

**Interfaces:**
- Consumes: Task 7 의 `tui.Labels`, `tui.Summary`.
- Produces:

```go
type Status string
const (
	StatusUpdated  Status = "updated"
	StatusUpToDate Status = "up-to-date"
	StatusSkipped  Status = "skipped"
	StatusFailed   Status = "failed"
)
type RepoResult struct {
	Name    string        `json:"name"`
	Trunk   string        `json:"trunk"`
	Status  Status        `json:"status"`
	Commits int           `json:"commits"`          // 이번에 합쳐진 커밋 수
	Reason  string        `json:"reason,omitempty"` // skipped/failed 사유
	Elapsed time.Duration `json:"elapsed_ns"`
	CostUSD float64       `json:"cost_usd"`
}
type RunResult struct {
	StartedAt  time.Time    `json:"started_at"`
	FinishedAt time.Time    `json:"finished_at"`
	Repos      []RepoResult `json:"repos"`
	LogPath    string       `json:"log_path"`
}
type Counts struct{ Updated, UpToDate, Skipped, Failed int; CostUSD float64 }
var Labels = tui.Labels{OK: "updated", Noop: "up to date", Skipped: "skipped", Failed: "failed"}
func (r RunResult) Counts() Counts
func (c Counts) String() string                 // tui.Summary(Labels, ...)
func (r RunResult) ExitCode() int               // skipped 나 failed 가 하나라도 있으면 1
func WriteRunResult(path string, r RunResult) error
func ReadRunResult(path string) (RunResult, error)

var ErrAlreadyRunning = errors.New("another ua-refresh run is in progress")
func AcquireLock(path string) (release func(), err error)

func SlackText(r RunResult) string              // 설계 6절 DM 본문
```

**스킬**: `superpowers:test-driven-development`.

**완료조건**: `go test ./internal/uarefresh/ -run 'Result|Lock|Slack|ExitCode'` PASS. DM 본문이 설계 6절 예시와 같은 구조(헤더 · 레포 줄 · `로그:` 줄)다.

- [x] **Step 1: 실패하는 테스트**

`internal/uarefresh/result_test.go`:

```go
package uarefresh

import (
	"path/filepath"
	"testing"
	"time"
)

func sampleRun() RunResult {
	return RunResult{
		StartedAt:  time.Date(2026, 9, 5, 8, 30, 0, 0, time.Local),
		FinishedAt: time.Date(2026, 9, 5, 8, 41, 0, 0, time.Local),
		Repos: []RepoResult{
			{Name: "example-api", Trunk: "main", Status: StatusUpdated, Commits: 13, Elapsed: 252 * time.Second, CostUSD: 1.83},
			{Name: "example-web", Trunk: "develop", Status: StatusUpdated, Commits: 2, Elapsed: 125 * time.Second, CostUSD: 0.71},
			{Name: "example-docs", Trunk: "main", Status: StatusUpToDate, Elapsed: 2 * time.Second},
			{Name: "example-infra", Trunk: "main", Status: StatusFailed, Reason: "ff-merge failed: 1 local commit ahead of origin", Elapsed: 3 * time.Second},
		},
		LogPath: "/home/example/Library/Logs/gofer/ua-refresh/2026-09-05.log",
	}
}

func TestCountsAndExitCode(t *testing.T) {
	r := sampleRun()
	if got := r.Counts().String(); got != "2 updated · 1 up to date · 0 skipped · 1 failed · $2.54" {
		t.Errorf("counts = %q", got)
	}
	if r.ExitCode() != 1 {
		t.Error("failed repo should give exit 1")
	}
	r.Repos = r.Repos[:3]
	if r.ExitCode() != 0 {
		t.Error("all good should give exit 0")
	}
	r.Repos[0].Status = StatusSkipped
	if r.ExitCode() != 1 {
		t.Error("skipped repo should give exit 1")
	}
}

func TestRunResultRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "last-run.json")
	if err := WriteRunResult(path, sampleRun()); err != nil {
		t.Fatal(err)
	}
	got, err := ReadRunResult(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Repos) != 4 || got.Repos[0].CostUSD != 1.83 || !got.FinishedAt.Equal(sampleRun().FinishedAt) {
		t.Errorf("round trip mismatch: %+v", got)
	}
}
```

`internal/uarefresh/lock_test.go`:

```go
package uarefresh

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestAcquireLockBlocksSecondRunner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "run.lock")
	release, err := AcquireLock(path)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if strings.TrimSpace(string(b)) != strconv.Itoa(os.Getpid()) {
		t.Errorf("lock should hold our pid, got %q", b)
	}
	if _, err := AcquireLock(path); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second acquire: want ErrAlreadyRunning, got %v", err)
	}
	release()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Error("release should remove the lock file")
	}
}

func TestAcquireLockRemovesStaleLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.lock")
	cmd := exec.Command("true") // 이미 끝난 프로세스의 pid 를 얻는다
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(path, []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0o644)
	release, err := AcquireLock(path)
	if err != nil {
		t.Fatalf("stale lock should be replaced, got %v", err)
	}
	defer release()
	b, _ := os.ReadFile(path)
	if strings.TrimSpace(string(b)) != strconv.Itoa(os.Getpid()) {
		t.Errorf("lock should now hold our pid, got %q", b)
	}
}
```

`internal/uarefresh/report_test.go`:

```go
package uarefresh

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSlackText(t *testing.T) {
	home, _ := os.UserHomeDir()
	r := sampleRun()
	r.LogPath = filepath.Join(home, "Library/Logs/gofer/ua-refresh/2026-09-05.log")
	want := `ua-refresh 2026-09-05 08:41 · 2 updated · 1 up to date · 0 skipped · 1 failed · $2.54
✓ example-api    main     +13 commits   4m12s  $1.83
✓ example-web    develop  +2 commits    2m05s  $0.71
– example-docs   main     up to date
✗ example-infra  main     ff-merge failed: 1 local commit ahead of origin
로그: ~/Library/Logs/gofer/ua-refresh/2026-09-05.log`
	if got := SlackText(r); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestSlackTextSkippedRow(t *testing.T) {
	r := RunResult{Repos: []RepoResult{{Name: "example-api", Trunk: "main", Status: StatusSkipped, Reason: "uncommitted changes in tracked files"}}}
	got := SlackText(r)
	if want := "↷ example-api  main  skipped: uncommitted changes in tracked files"; !strings.Contains(got, want) {
		t.Errorf("got:\n%s\nwant line:\n%s", got, want)
	}
}
```

(`report_test.go` 의 import 에 `"strings"` 를 포함한다.)

- [x] **Step 2: 실패 확인**

Run: `go test ./internal/uarefresh/ -run 'Result|Lock|Slack|ExitCode'`
Expected: 컴파일 실패.

- [x] **Step 3: 구현**

`internal/uarefresh/result.go`:

```go
package uarefresh

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"gofer/internal/tui"
)

// Status 는 레포 하나의 결과다 (설계 4절).
type Status string

const (
	StatusUpdated  Status = "updated"    // 그래프 갱신
	StatusUpToDate Status = "up-to-date" // 변경 없음, Claude 호출 안 함
	StatusSkipped  Status = "skipped"    // 가드에 걸림
	StatusFailed   Status = "failed"     // merge·Claude·검증 실패
)

// RepoResult 는 레포 하나의 처리 결과다. last-run.json 에 그대로 저장된다.
type RepoResult struct {
	Name    string        `json:"name"`
	Trunk   string        `json:"trunk"`
	Status  Status        `json:"status"`
	Commits int           `json:"commits"`
	Reason  string        `json:"reason,omitempty"`
	Elapsed time.Duration `json:"elapsed_ns"`
	CostUSD float64       `json:"cost_usd"`
}

// RunResult 는 한 번의 실행 전체다.
type RunResult struct {
	StartedAt  time.Time    `json:"started_at"`
	FinishedAt time.Time    `json:"finished_at"`
	Repos      []RepoResult `json:"repos"`
	LogPath    string       `json:"log_path"`
}

// Labels 는 화면·DM 요약에 쓰는 결과별 문구다.
var Labels = tui.Labels{OK: "updated", Noop: "up to date", Skipped: "skipped", Failed: "failed"}

// Counts 는 결과별 개수와 총비용이다.
type Counts struct {
	Updated, UpToDate, Skipped, Failed int
	CostUSD                            float64
}

func (r RunResult) Counts() Counts {
	var c Counts
	for _, rr := range r.Repos {
		switch rr.Status {
		case StatusUpdated:
			c.Updated++
		case StatusUpToDate:
			c.UpToDate++
		case StatusSkipped:
			c.Skipped++
		case StatusFailed:
			c.Failed++
		}
		c.CostUSD += rr.CostUSD
	}
	return c
}

func (c Counts) String() string {
	return tui.Summary(Labels, c.Updated, c.UpToDate, c.Skipped, c.Failed, c.CostUSD)
}

// ExitCode 는 skipped 나 failed 가 하나라도 있으면 1 이다 (설계 4절).
func (r RunResult) ExitCode() int {
	c := r.Counts()
	if c.Skipped > 0 || c.Failed > 0 {
		return 1
	}
	return 0
}

// WriteRunResult 는 last-run.json 을 쓴다.
func WriteRunResult(path string, r RunResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// ReadRunResult 는 last-run.json 을 읽는다.
func ReadRunResult(path string) (RunResult, error) {
	var r RunResult
	b, err := os.ReadFile(path)
	if err != nil {
		return r, err
	}
	return r, json.Unmarshal(b, &r)
}
```

`internal/uarefresh/lock.go`:

```go
package uarefresh

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// ErrAlreadyRunning 은 살아 있는 다른 실행이 잠금을 쥐고 있을 때다.
var ErrAlreadyRunning = errors.New("another ua-refresh run is in progress")

// AcquireLock 은 pid 를 담은 잠금 파일을 만든다. 파일이 있어도 그 pid 가 죽어 있으면 stale 로 보고 지운다 (설계 7절).
func AcquireLock(path string) (release func(), err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			fmt.Fprintf(f, "%d\n", os.Getpid())
			f.Close()
			return func() { os.Remove(path) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil, rerr
		}
		pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
		if pid > 0 && processAlive(pid) {
			return nil, fmt.Errorf("%w (pid %d)", ErrAlreadyRunning, pid)
		}
		os.Remove(path) // stale — 다음 시도에서 새로 만든다
	}
	return nil, ErrAlreadyRunning
}

// processAlive 는 시그널 0 으로 생존을 확인한다. 권한 오류(EPERM)는 살아 있다는 뜻이다.
func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
```

`internal/uarefresh/report.go`:

```go
package uarefresh

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gofer/internal/tui"
)

// SlackText 는 실행 종료 시 보내는 DM 본문이다 (설계 6절). 항상 1건, 헤더 · 레포별 한 줄 · 로그 경로.
func SlackText(r RunResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "ua-refresh %s · %s\n", r.FinishedAt.Format("2006-01-02 15:04"), r.Counts())
	nw, tw := 0, 0
	for _, rr := range r.Repos {
		nw, tw = max(nw, len(rr.Name)), max(tw, len(rr.Trunk))
	}
	for _, rr := range r.Repos {
		fmt.Fprintf(&b, "%s %-*s  %-*s  %s\n", statusSymbol(rr.Status), nw, rr.Name, tw, rr.Trunk, repoLine(rr))
	}
	fmt.Fprintf(&b, "로그: %s", shortenHome(r.LogPath))
	return b.String()
}

func statusSymbol(s Status) string {
	switch s {
	case StatusUpdated:
		return tui.OutcomeOK.Symbol()
	case StatusUpToDate:
		return tui.OutcomeNoop.Symbol()
	case StatusSkipped:
		return tui.OutcomeSkipped.Symbol()
	default:
		return tui.OutcomeFailed.Symbol()
	}
}

// repoLine 은 레포 줄의 상태 부분이다.
func repoLine(rr RepoResult) string {
	switch rr.Status {
	case StatusUpdated:
		return strings.TrimRight(fmt.Sprintf("%-12s  %s  $%.2f", commitsText(rr.Commits), rr.Elapsed.Round(time.Second), rr.CostUSD), " ")
	case StatusUpToDate:
		return "up to date"
	case StatusSkipped:
		return "skipped: " + rr.Reason
	default:
		return rr.Reason
	}
}

// commitsText 는 "+13 commits" 다. 0 이면 빈 문자열.
func commitsText(n int) string {
	switch n {
	case 0:
		return ""
	case 1:
		return "+1 commit"
	default:
		return fmt.Sprintf("+%d commits", n)
	}
}

// shortenHome 은 홈 디렉토리를 ~ 로 줄인다.
func shortenHome(p string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(p, home+"/") {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}
```

- [x] **Step 4: 통과 확인**

Run: `go test ./internal/uarefresh/ && go vet ./...`
Expected: PASS.

- [x] **Step 5: 커밋**

```bash
git add internal/uarefresh/result.go internal/uarefresh/result_test.go internal/uarefresh/lock.go internal/uarefresh/lock_test.go internal/uarefresh/report.go internal/uarefresh/report_test.go
git commit -m "feat(ua-refresh): run results, lock file and slack report text"
```

- [x] **Step 6: `/demiurge:rl` 로 완료조건 검증**

---

### Task 9: 오케스트레이터 — 레포 순차 처리와 Event 발행

**Files:**
- Create: `internal/uarefresh/orchestrator.go`, `internal/uarefresh/orchestrator_test.go`

**Interfaces:**
- Consumes: Task 4 git 함수들, Task 5 `DecideGraph`/`GraphHash`, Task 6 `RunClaude`/`ClaudeOptions`, Task 7 `tui.Event`, Task 8 `RepoResult`/`RunResult`/`commitsText`. 테스트는 `newRepoWithOrigin`·`pushFromClone`·`commitFile`·`writeMeta`·`installFakeClaude`.
- Produces:

```go
type RunParams struct {
	Repos     []RepoConfig
	Claude    ClaudeConfig
	ExtraPath []string          // ResolveExtraPath 결과
	Log       io.Writer         // 상세 로그. nil 이면 버림
	Events    chan<- tui.Event  // Run 이 KindAllDone 뒤에 닫는다
	Now       func() time.Time  // nil 이면 time.Now
}
func Run(ctx context.Context, p RunParams) RunResult          // LogPath 는 채우지 않는다 (호출자가 채움)
func GuardReason(ctx context.Context, repo RepoConfig) (reason string, err error) // "" 면 통과. dry-run 도 사용
```

Event 규약 (Task 11 의 화면과 Task 12 의 dry-run 이 의존):
- 레포마다 `KindStart` → `KindStage{Label:"fetch"}` → `KindStage{Label:"merge", Detail:"+N commits"}` → (Claude 를 부르면) `KindStage{Label:"/understand …" 또는 "/understand --full …", Detail}` → `KindDone`. 가드에 걸리면 `KindStart` 바로 뒤 `KindDone`.
- `KindDone` 의 매핑: updated → `OutcomeOK`, Label `"graph updated"`, Detail `"+N commits"`, Elapsed, CostUSD / up-to-date → `OutcomeNoop`, Label `"up to date"` / skipped → `OutcomeSkipped`, Label `"skipped: <사유>"` / failed → `OutcomeFailed`, Label `<사유>`, Elapsed, CostUSD.

**스킬**: `superpowers:test-driven-development`. 완료 후 executing-plans 로 실행 중이면 `/code-review` 1회.

**완료조건**: `go test ./internal/uarefresh/ -run 'Run[A-Z]|Guard'` PASS — 최신(Claude 미호출) · 갱신 · `--full` · 브랜치 가드 · 미커밋 가드 · ff 실패(Claude 미호출) · 해시 미갱신 실패 · is_error 실패 · 실패 후 다음 레포 계속. 오케스트레이터 파일에 `bubbletea`·`lipgloss`·`fmt.Print` 가 없다(화면을 모른다).

- [x] **Step 1: 실패하는 테스트**

`internal/uarefresh/orchestrator_test.go`:

```go
package uarefresh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gofer/internal/tui"
)

// runRepos 는 Run 을 돌리고 결과와 Event 목록을 돌려준다.
func runRepos(t *testing.T, f fakeClaude, repos ...RepoConfig) (RunResult, []tui.Event) {
	t.Helper()
	events := make(chan tui.Event, 64)
	res := Run(t.Context(), RunParams{
		Repos:     repos,
		Claude:    ClaudeConfig{BudgetUSD: 5, TimeoutMin: 1},
		ExtraPath: []string{f.Dir},
		Events:    events,
	})
	var evs []tui.Event
	for ev := range events {
		evs = append(evs, ev)
	}
	return res, evs
}

func runOne(t *testing.T, f fakeClaude, work, trunk string) (RepoResult, []tui.Event) {
	t.Helper()
	res, evs := runRepos(t, f, RepoConfig{Path: work, Trunk: trunk})
	if len(res.Repos) != 1 {
		t.Fatalf("repos = %d", len(res.Repos))
	}
	return res.Repos[0], evs
}

// trace 는 Event 목록을 "start stage:fetch ... done:✓ alldone" 꼴로 요약한다.
func trace(evs []tui.Event) string {
	var parts []string
	for _, ev := range evs {
		switch ev.Kind {
		case tui.KindStart:
			parts = append(parts, "start")
		case tui.KindStage:
			parts = append(parts, "stage:"+ev.Label)
		case tui.KindDone:
			parts = append(parts, "done:"+ev.Outcome.Symbol())
		case tui.KindAllDone:
			parts = append(parts, "alldone")
		}
	}
	return strings.Join(parts, " ")
}

func TestRunUpToDateDoesNotCallClaude(t *testing.T) {
	work, _ := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "ok")
	head, _ := Head(t.Context(), work)
	writeMeta(t, work, ".ua", head)
	rr, evs := runOne(t, f, work, "main")
	if rr.Status != StatusUpToDate || rr.CostUSD != 0 {
		t.Fatalf("result = %+v", rr)
	}
	if f.called() {
		t.Fatal("claude must not run when graph hash == HEAD")
	}
	if got := trace(evs); got != "start stage:fetch stage:merge done:– alldone" {
		t.Errorf("trace = %q", got)
	}
}

func TestRunUpdatesWhenOriginAhead(t *testing.T) {
	work, origin := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "ok")
	want := pushFromClone(t, origin, "main", "a.txt", "a")
	rr, evs := runOne(t, f, work, "main")
	if rr.Status != StatusUpdated || rr.Commits != 1 || rr.CostUSD != 0.5 || rr.Elapsed <= 0 {
		t.Fatalf("result = %+v", rr)
	}
	if h, _ := Head(t.Context(), work); h != want {
		t.Errorf("HEAD = %s want %s (ff-merge did not happen)", h, want)
	}
	if args := f.argsLines(t); args[1] != "/understand" {
		t.Errorf("prompt = %q", args[1])
	}
	if got := trace(evs); got != "start stage:fetch stage:merge stage:/understand … done:✓ alldone" {
		t.Errorf("trace = %q", got)
	}
	done := evs[len(evs)-2]
	if done.Label != "graph updated" || done.Detail != "+1 commit" || done.CostUSD != 0.5 || done.Elapsed <= 0 {
		t.Errorf("done event = %+v", done)
	}
}

func TestRunUsesFullWhenGraphCommitMissing(t *testing.T) {
	work, _ := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "ok")
	writeMeta(t, work, ".ua", "0123456789abcdef0123456789abcdef01234567")
	rr, evs := runOne(t, f, work, "main")
	if rr.Status != StatusUpdated {
		t.Fatalf("result = %+v", rr)
	}
	if args := f.argsLines(t); args[1] != "/understand --full" {
		t.Errorf("prompt = %q", args[1])
	}
	if !strings.Contains(trace(evs), "stage:/understand --full …") {
		t.Errorf("trace = %q", trace(evs))
	}
}

func TestRunSkipsWhenBranchMismatch(t *testing.T) {
	work, _ := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "ok")
	run(t, work, "git", "checkout", "-q", "-b", "feature-x")
	rr, evs := runOne(t, f, work, "main")
	if rr.Status != StatusSkipped || !strings.Contains(rr.Reason, `"feature-x"`) {
		t.Fatalf("result = %+v", rr)
	}
	if f.called() || strings.Contains(trace(evs), "fetch") {
		t.Fatalf("guard must stop before fetch/claude; trace = %q", trace(evs))
	}
}

func TestRunSkipsWhenTrackedChanges(t *testing.T) {
	work, _ := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "ok")
	os.WriteFile(filepath.Join(work, "README.md"), []byte("edited"), 0o644)
	rr, _ := runOne(t, f, work, "main")
	if rr.Status != StatusSkipped || rr.Reason != "uncommitted changes in tracked files" {
		t.Fatalf("result = %+v", rr)
	}
}

func TestRunFailsWhenDiverged(t *testing.T) {
	work, origin := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "ok")
	commitFile(t, work, "local.txt", "l")
	pushFromClone(t, origin, "main", "remote.txt", "r")
	rr, _ := runOne(t, f, work, "main")
	if rr.Status != StatusFailed || rr.Reason != "ff-merge failed: 1 local commit ahead of origin" {
		t.Fatalf("result = %+v", rr)
	}
	if f.called() {
		t.Fatal("claude must not run after merge failure")
	}
}

func TestRunFailsWhenGraphHashNotUpdated(t *testing.T) {
	work, origin := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "noop")
	pushFromClone(t, origin, "main", "a.txt", "a")
	rr, _ := runOne(t, f, work, "main")
	if rr.Status != StatusFailed || !strings.Contains(rr.Reason, "graph hash still not at HEAD") || rr.CostUSD != 0.1 {
		t.Fatalf("result = %+v", rr)
	}
}

func TestRunFailsWhenClaudeReportsError(t *testing.T) {
	work, origin := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "error")
	pushFromClone(t, origin, "main", "a.txt", "a")
	rr, _ := runOne(t, f, work, "main")
	if rr.Status != StatusFailed || !strings.Contains(rr.Reason, "budget exceeded") || rr.CostUSD != 0.2 {
		t.Fatalf("result = %+v", rr)
	}
}

func TestRunContinuesAfterFailure(t *testing.T) {
	bad, badOrigin := newRepoWithOrigin(t, "main")
	commitFile(t, bad, "local.txt", "l")
	pushFromClone(t, badOrigin, "main", "remote.txt", "r")
	good, _ := newRepoWithOrigin(t, "develop")
	head, _ := Head(t.Context(), good)
	writeMeta(t, good, ".ua", head)
	f := installFakeClaude(t, "ok")
	res, evs := runRepos(t, f, RepoConfig{Path: bad, Trunk: "main"}, RepoConfig{Path: good, Trunk: "develop"})
	if res.Repos[0].Status != StatusFailed || res.Repos[1].Status != StatusUpToDate {
		t.Fatalf("results = %+v", res.Repos)
	}
	if res.ExitCode() != 1 {
		t.Error("exit code should be 1")
	}
	if got := trace(evs); got != "start stage:fetch stage:merge done:✗ start stage:fetch stage:merge done:– alldone" {
		t.Errorf("trace = %q", got)
	}
}

func TestGuardReasonPasses(t *testing.T) {
	work, _ := newRepoWithOrigin(t, "main")
	reason, err := GuardReason(t.Context(), RepoConfig{Path: work, Trunk: "main"})
	if err != nil || reason != "" {
		t.Fatalf("got (%q, %v)", reason, err)
	}
}
```

- [x] **Step 2: 실패 확인**

Run: `go test ./internal/uarefresh/ -run 'Run[A-Z]|Guard'`
Expected: 컴파일 실패 (`undefined: Run`).

- [x] **Step 3: 구현**

`internal/uarefresh/orchestrator.go`:

```go
package uarefresh

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"gofer/internal/tui"
)

// RunParams 는 한 번의 실행에 필요한 것들이다. 화면은 모른다 — Events 로만 알린다 (설계 5절).
type RunParams struct {
	Repos     []RepoConfig
	Claude    ClaudeConfig
	ExtraPath []string
	Log       io.Writer
	Events    chan<- tui.Event
	Now       func() time.Time
}

// Run 은 설정된 레포를 순서대로 처리한다 (설계 4절 전체 흐름). 한 레포의 실패가 다음 레포를 막지 않는다.
// 마지막에 KindAllDone 을 보내고 Events 를 닫는다.
func Run(ctx context.Context, p RunParams) RunResult {
	now := p.Now
	if now == nil {
		now = time.Now
	}
	logOut := p.Log
	if logOut == nil {
		logOut = io.Discard
	}
	logger := log.New(logOut, "", log.LstdFlags)

	res := RunResult{StartedAt: now()}
	for i, repo := range p.Repos {
		p.Events <- tui.Event{Kind: tui.KindStart, Index: i}
		rr := processRepo(ctx, p, logger, i, repo, now)
		res.Repos = append(res.Repos, rr)
		p.Events <- doneEvent(i, rr)
	}
	res.FinishedAt = now()
	p.Events <- tui.Event{Kind: tui.KindAllDone}
	close(p.Events)
	return res
}

// GuardReason 은 설계 4절 ① 가드다. 건너뛸 사유를 돌려주고, 통과하면 "" 다. dry-run 도 같은 판정을 쓴다.
func GuardReason(ctx context.Context, repo RepoConfig) (string, error) {
	branch, err := CurrentBranch(ctx, repo.Path)
	if err != nil {
		return "", err
	}
	if branch != repo.Trunk {
		if branch == "" {
			branch = "detached HEAD"
		}
		return fmt.Sprintf("branch is %q, expected %q", branch, repo.Trunk), nil
	}
	dirty, err := HasTrackedChanges(ctx, repo.Path)
	if err != nil {
		return "", err
	}
	if dirty {
		return "uncommitted changes in tracked files", nil
	}
	return "", nil
}

// processRepo 는 레포 하나를 ①가드 → ②fetch → ③ff-merge → ④해시 비교 → ⑤claude → ⑥검증 순으로 처리한다.
func processRepo(ctx context.Context, p RunParams, logger *log.Logger, i int, repo RepoConfig, now func() time.Time) (rr RepoResult) {
	start := now()
	rr = RepoResult{Name: repo.Name(), Trunk: repo.Trunk}
	defer func() { rr.Elapsed = now().Sub(start) }()

	stage := func(label, detail string) {
		p.Events <- tui.Event{Kind: tui.KindStage, Index: i, Label: label, Detail: detail}
	}
	fail := func(reason string) RepoResult {
		rr.Status, rr.Reason = StatusFailed, reason
		logger.Printf("[%s] failed: %s", rr.Name, reason)
		return rr
	}

	// ① 가드: 루트 작업 트리는 항상 trunk 여야 한다. 아니면 checkout 하지 않고 건너뛴다 (설계 2·4절).
	reason, err := GuardReason(ctx, repo)
	if err != nil {
		return fail(err.Error())
	}
	if reason != "" {
		rr.Status, rr.Reason = StatusSkipped, reason
		logger.Printf("[%s] skipped: %s", rr.Name, reason)
		return rr
	}

	// ② fetch
	stage("fetch", "")
	out, err := Fetch(ctx, repo.Path)
	if err != nil {
		return fail(err.Error())
	}
	if out != "" {
		logger.Printf("[%s] fetch:\n%s", rr.Name, out)
	}

	// ③ ff-merge
	behind, err := CountCommits(ctx, repo.Path, "HEAD", "origin/"+repo.Trunk)
	if err != nil {
		return fail(err.Error())
	}
	rr.Commits = behind
	detail := commitsText(behind)
	stage("merge", detail)
	if err := FFMerge(ctx, repo.Path, repo.Trunk); err != nil {
		if ahead, _ := CountCommits(ctx, repo.Path, "origin/"+repo.Trunk, "HEAD"); ahead > 0 {
			return fail(fmt.Sprintf("ff-merge failed: %s ahead of origin", localCommitsText(ahead)))
		}
		return fail("ff-merge failed: " + err.Error())
	}

	// ④ 그래프 해시 == HEAD 면 Claude 를 부르지 않는다 (/understand 가 되묻고 멈추므로).
	head, err := Head(ctx, repo.Path)
	if err != nil {
		return fail(err.Error())
	}
	decision, graphHash, err := DecideGraph(ctx, repo.Path, head)
	if err != nil {
		return fail(err.Error())
	}
	if decision == GraphUpToDate {
		rr.Status = StatusUpToDate
		logger.Printf("[%s] up to date at %.7s", rr.Name, head)
		return rr
	}

	// ⑤ claude
	stage(decision.String()+" …", detail)
	logger.Printf("[%s] %s (graph %.7s → HEAD %.7s)", rr.Name, decision, graphHash, head)
	cres, cerr := RunClaude(ctx, ClaudeOptions{
		Dir: repo.Path, Full: decision == GraphFull,
		BudgetUSD: p.Claude.BudgetUSD, Timeout: p.Claude.Timeout(), Model: p.Claude.Model,
		ExtraPath: p.ExtraPath, OAuthToken: p.Claude.OAuthToken, Stderr: p.Log,
	})
	rr.CostUSD = cres.TotalCostUSD
	logger.Printf("[%s] claude: is_error=%v cost=$%.2f timed_out=%v err=%v", rr.Name, cres.IsError, cres.TotalCostUSD, cres.TimedOut, cerr)

	// ⑥ 검증: 종료 코드가 아니라 그래프 해시가 HEAD 가 됐는지로 판정한다 (설계 4절).
	if after, err := GraphHash(repo.Path); err == nil && after == head {
		rr.Status = StatusUpdated
		return rr
	}
	if cerr != nil {
		return fail(cerr.Error())
	}
	return fail(fmt.Sprintf("graph hash still not at HEAD %.7s after %s", head, decision))
}

// doneEvent 는 RepoResult 를 화면용 완료 Event 로 바꾼다.
func doneEvent(i int, rr RepoResult) tui.Event {
	ev := tui.Event{Kind: tui.KindDone, Index: i}
	switch rr.Status {
	case StatusUpdated:
		ev.Outcome, ev.Label, ev.Detail = tui.OutcomeOK, "graph updated", commitsText(rr.Commits)
		ev.Elapsed, ev.CostUSD = rr.Elapsed, rr.CostUSD
	case StatusUpToDate:
		ev.Outcome, ev.Label = tui.OutcomeNoop, "up to date"
	case StatusSkipped:
		ev.Outcome, ev.Label = tui.OutcomeSkipped, "skipped: "+rr.Reason
	default:
		ev.Outcome, ev.Label = tui.OutcomeFailed, rr.Reason
		ev.Elapsed, ev.CostUSD = rr.Elapsed, rr.CostUSD
	}
	return ev
}

func localCommitsText(n int) string {
	if n == 1 {
		return "1 local commit"
	}
	return fmt.Sprintf("%d local commits", n)
}
```

- [x] **Step 4: 통과 확인**

Run: `go test ./internal/uarefresh/ && go vet ./... && grep -n -E 'bubbletea|lipgloss|fmt\.Print' internal/uarefresh/orchestrator.go; echo "grep exit=$?"`
Expected: PASS, `grep exit=1`.

- [x] **Step 5: 커밋**

```bash
git add internal/uarefresh/orchestrator.go internal/uarefresh/orchestrator_test.go
git commit -m "feat(ua-refresh): orchestrate repos sequentially and emit progress events"
```

- [x] **Step 6: `/demiurge:rl` 로 완료조건 검증** (executing-plans 로 실행 중이면 이어서 `/code-review`)

---

### Task 10: Slack 웹훅 · macOS 알림

**Files:**
- Create: `internal/slack/webhook.go`, `internal/slack/webhook_test.go`, `internal/uarefresh/notify.go`, `internal/uarefresh/notify_test.go`

**Interfaces:**
- Consumes: 없음
- Produces:

```go
// 패키지 gofer/internal/slack (도구 공용)
func Post(ctx context.Context, webhookURL, text string) error   // {"text": text} POST. 2xx 아니면 상태·본문을 담은 오류

// 패키지 gofer/internal/uarefresh
func MacNotify(ctx context.Context, title, message string) error // osascript -e 'display notification "..." with title "..."'
```

**스킬**: `superpowers:test-driven-development`.

**완료조건**: `go test ./internal/slack/ && go test ./internal/uarefresh/ -run Notify` PASS — 요청 본문·Content-Type 확인, 5xx 오류 문구, osascript 인자와 따옴표 이스케이프.

- [x] **Step 1: 실패하는 테스트**

`internal/slack/webhook_test.go`:

```go
package slack

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPostSendsTextAsJSON(t *testing.T) {
	var gotBody, gotType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody, gotType = string(b), r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "ok")
	}))
	defer srv.Close()
	if err := Post(t.Context(), srv.URL, "line 1\nline \"2\""); err != nil {
		t.Fatal(err)
	}
	if gotType != "application/json" || gotBody != `{"text":"line 1\nline \"2\""}` {
		t.Errorf("type=%q body=%q", gotType, gotBody)
	}
}

func TestPostReportsNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, "invalid_payload")
	}))
	defer srv.Close()
	err := Post(t.Context(), srv.URL, "x")
	if err == nil || !strings.Contains(err.Error(), "500") || !strings.Contains(err.Error(), "invalid_payload") {
		t.Fatalf("got %v", err)
	}
}
```

`internal/uarefresh/notify_test.go`:

```go
package uarefresh

import (
	"os"
	"path/filepath"
	"testing"
)

// installFakeOsascript 는 PATH 맨 앞에 인자를 기록하는 가짜 osascript 를 둔다. (Task 12 도 재사용)
func installFakeOsascript(t *testing.T) (argsFile string) {
	t.Helper()
	dir := t.TempDir()
	argsFile = filepath.Join(dir, "args")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"" + argsFile + "\"\n"
	if err := os.WriteFile(filepath.Join(dir, "osascript"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argsFile
}

func TestMacNotifyBuildsAppleScript(t *testing.T) {
	argsFile := installFakeOsascript(t)
	if err := MacNotify(t.Context(), "ua-refresh", `2 updated · 1 failed "quoted"`); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(argsFile)
	want := "-e\ndisplay notification \"2 updated · 1 failed \\\"quoted\\\"\" with title \"ua-refresh\"\n"
	if string(b) != want {
		t.Errorf("args:\n%s\nwant:\n%s", b, want)
	}
}
```

- [x] **Step 2: 실패 확인**

Run: `go test ./internal/slack/ ./internal/uarefresh/ -run 'Post|Notify'`
Expected: 컴파일 실패.

- [x] **Step 3: 구현**

`internal/slack/webhook.go`:

```go
// Package slack 은 Incoming Webhook 으로 메시지를 보낸다. 도구 공용.
package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Post 는 webhookURL 로 text 한 건을 보낸다. 2xx 가 아니면 상태와 본문을 담은 오류를 돌려준다.
func Post(ctx context.Context, webhookURL, text string) error {
	body, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("slack webhook: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("slack webhook: %s: %s", resp.Status, bytes.TrimSpace(b))
	}
	return nil
}
```

`internal/uarefresh/notify.go`:

```go
package uarefresh

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// MacNotify 는 macOS 알림 한 줄이다. Slack 전송이 실패했을 때의 대체 수단 (설계 6·7절).
func MacNotify(ctx context.Context, title, message string) error {
	script := fmt.Sprintf("display notification %s with title %s", appleScriptString(message), appleScriptString(title))
	return exec.CommandContext(ctx, "osascript", "-e", script).Run()
}

// appleScriptString 은 AppleScript 문자열 리터럴로 감싼다.
func appleScriptString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}
```

- [x] **Step 4: 통과 확인**

Run: `go test ./internal/slack/ ./internal/uarefresh/ && go vet ./...`
Expected: PASS.

- [x] **Step 5: 커밋**

```bash
git add internal/slack/ internal/uarefresh/notify.go internal/uarefresh/notify_test.go
git commit -m "feat: slack incoming webhook client and macOS notification fallback"
```

- [x] **Step 6: `/demiurge:rl` 로 완료조건 검증**

---

### Task 11: 터미널 화면 — bubbletea 모델과 소비자 선택

**Files:**
- Create: `internal/tui/model.go`, `internal/tui/model_test.go`, `internal/tui/run.go`

**Interfaces:**
- Consumes: Task 7 의 `Event`, `Item`, `Labels`, `tally`, `elapsedText`, `costText`, `Plain`.
- Produces (패키지 `gofer/internal/tui`):

```go
type EventMsg Event                                   // Event 를 bubbletea 메시지로 감싼 것
type Model struct{ /* 비공개 */ }
func NewModel(title string, items []Item, labels Labels, now func() time.Time) Model
func (m Model) Init() tea.Cmd
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) // EventMsg·spinner.TickMsg·tickMsg·ctrl+c
func (m Model) View() tea.View                       // tea.NewView(m.Render())
func (m *Model) Apply(ev Event)                      // Program 없이 상태 반영 (테스트·Update 가 사용)
func (m Model) Render() string                       // 화면 문자열 (골든 테스트 대상)
func (m Model) Done() bool

type Options struct {
	Title  string           // 헤더. 예: "ua-refresh · 6 repos"
	Items  []Item
	Labels Labels
	Out    io.Writer        // 화면 (보통 os.Stdout)
	Log    io.Writer        // plain 한 줄 로그를 항상 받는 곳. nil 이면 없음
	TTY    bool             // true 면 bubbletea 화면, 아니면 Out 에도 plain
	Now    func() time.Time // nil 이면 time.Now
}
var ErrInterrupted = errors.New("interrupted by user")
func IsTerminal(f *os.File) bool
func Run(ctx context.Context, o Options, events <-chan Event) error
```

`Run` 규약: events 가 닫힐 때까지 소비한다. TTY 가 아니면 `Out` 과 `Log` 에 한 줄씩 쓰고 nil 을 돌려준다. TTY 면 `Log` 에만 한 줄씩 쓰면서 화면을 띄우고, `KindAllDone` 이 오면 마지막 화면을 남기고 끝낸다. 사용자가 ctrl+c 를 누르면 즉시 `ErrInterrupted` 를 돌려주며 **남은 events 는 백그라운드에서 계속 `Log` 로 흘린다** — 호출자는 ctx 를 취소하고 처리기가 끝나기를 기다린 뒤 로그 파일을 닫아야 한다.

**스킬**: `superpowers:test-driven-development`. 화면 미세 조정이 필요하면 `superpowers:verification-before-completion` 전에 실제 터미널에서 Step 5 의 수동 확인을 한다.

**완료조건**: `go test ./internal/tui/` PASS(골든 렌더링 2종 + AllDone → Quit). Step 5 의 수동 확인에서 스피너가 돌고, 끝난 뒤 요약 줄이 터미널에 남아 있다. `internal/tui` 의 비테스트 파일에 `repo|graph|understand` 가 없다.

- [x] **Step 1: 실패하는 테스트**

`internal/tui/model_test.go`:

```go
package tui

import (
	"regexp"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func newTestModel(now *time.Time) Model {
	items := []Item{{"example-api", "main"}, {"example-web", "develop"}, {"example-docs", "main"}}
	return NewModel("ua-refresh · 3 repos", items, testLabels, func() time.Time { return *now })
}

func TestRenderWhileRunning(t *testing.T) {
	now := fixedNow()
	m := newTestModel(&now)
	m.Apply(Event{Kind: KindStart, Index: 0})
	m.Apply(Event{Kind: KindDone, Index: 0, Outcome: OutcomeOK, Label: "graph updated", Detail: "+13 commits", Elapsed: 252 * time.Second, CostUSD: 1.83})
	m.Apply(Event{Kind: KindStart, Index: 1})
	m.Apply(Event{Kind: KindStage, Index: 1, Label: "merge", Detail: "+2 commits"})
	m.Apply(Event{Kind: KindStage, Index: 1, Label: "/understand …"}) // Detail 이 비면 이전 값 유지
	now = now.Add(97 * time.Second)
	want := ` ua-refresh · 3 repos · 08:41:02

 ✓ example-api   main     +13 commits   graph updated      4m12s  $1.83
 ⠋ example-web   develop  +2 commits    /understand …      1m37s
 · example-docs  main     waiting

 ─────────────────────────────────────────────────────────────────
 1 updated · 0 up to date · 0 skipped · 0 failed · $1.83 so far
`
	if got := ansiRE.ReplaceAllString(m.Render(), ""); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderWhenDone(t *testing.T) {
	now := fixedNow()
	m := newTestModel(&now)
	m.Apply(Event{Kind: KindStart, Index: 0})
	m.Apply(Event{Kind: KindDone, Index: 0, Outcome: OutcomeNoop, Label: "up to date"})
	m.Apply(Event{Kind: KindStart, Index: 1})
	m.Apply(Event{Kind: KindDone, Index: 1, Outcome: OutcomeSkipped, Label: `skipped: branch is "feature-x", expected "develop"`})
	m.Apply(Event{Kind: KindStart, Index: 2})
	m.Apply(Event{Kind: KindDone, Index: 2, Outcome: OutcomeFailed, Label: "ff-merge failed: 1 local commit ahead of origin", Elapsed: 3 * time.Second})
	m.Apply(Event{Kind: KindAllDone})
	want := ` ua-refresh · 3 repos · 08:41:02

 – example-api   main     up to date
 ↷ example-web   develop  skipped: branch is "feature-x", expected "develop"
 ✗ example-docs  main     ff-merge failed: 1 local commit ahead of origin      3s

 ─────────────────────────────────────────────────────────────────
 0 updated · 1 up to date · 1 skipped · 1 failed · $0.00
`
	if got := ansiRE.ReplaceAllString(m.Render(), ""); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if !m.Done() {
		t.Error("Done() should be true after KindAllDone")
	}
}

func TestUpdateQuitsOnAllDone(t *testing.T) {
	now := fixedNow()
	m := newTestModel(&now)
	next, cmd := m.Update(EventMsg{Kind: KindAllDone})
	if cmd == nil {
		t.Fatal("expected a command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("expected QuitMsg, got %T", cmd())
	}
	if !next.(Model).Done() {
		t.Error("model should be done")
	}
}

func TestUpdateInterruptsOnCtrlC(t *testing.T) {
	now := fixedNow()
	m := newTestModel(&now)
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("expected a command")
	}
	if _, ok := cmd().(tea.InterruptMsg); !ok {
		t.Errorf("expected InterruptMsg, got %T", cmd())
	}
}
```

- [x] **Step 2: 실패 확인**

Run: `go test ./internal/tui/`
Expected: 컴파일 실패 (`undefined: NewModel`).

- [x] **Step 3: 구현**

```bash
go get charm.land/bubbletea/v2@v2.0.9 charm.land/bubbles/v2@v2.2.1 charm.land/lipgloss/v2@v2.0.6
```

`internal/tui/model.go`:

```go
package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// EventMsg 는 Event 를 bubbletea 메시지로 감싼 것이다.
type EventMsg Event

// tickMsg 는 1초마다 경과 시간을 다시 그리기 위한 신호다.
type tickMsg time.Time

type rowPhase int

const (
	rowWaiting rowPhase = iota
	rowRunning
	rowDone
)

type row struct {
	phase   rowPhase
	started time.Time
	label   string
	detail  string
	outcome Outcome
	elapsed time.Duration
	cost    float64
}

var (
	styleOK     = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	styleFailed = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	styleDim    = lipgloss.NewStyle().Faint(true)
)

// Model 은 "항목마다 한 줄 + 요약 줄" 화면이다 (설계 6절).
type Model struct {
	title  string
	items  []Item
	labels Labels
	now    func() time.Time
	start  time.Time
	rows   []row
	tally  tally
	sp     spinner.Model
	done   bool
	nameW  int
	subW   int
}

// NewModel 은 모든 항목이 waiting 인 화면을 만든다.
func NewModel(title string, items []Item, labels Labels, now func() time.Time) Model {
	m := Model{
		title: title, items: items, labels: labels, now: now, start: now(),
		rows: make([]row, len(items)),
		sp:   spinner.New(spinner.WithSpinner(spinner.MiniDot)),
	}
	for _, it := range items {
		m.nameW, m.subW = max(m.nameW, len(it.Name)), max(m.subW, len(it.Sub))
	}
	return m
}

func (m Model) Init() tea.Cmd { return tea.Batch(m.sp.Tick, tick()) }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Interrupt
		}
	case EventMsg:
		m.Apply(Event(msg))
		if m.done {
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.sp, cmd = m.sp.Update(msg)
		return m, cmd
	case tickMsg:
		return m, tick()
	}
	return m, nil
}

func (m Model) View() tea.View { return tea.NewView(m.Render()) }

// Done 은 KindAllDone 을 받았는지다.
func (m Model) Done() bool { return m.done }

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// Apply 는 Event 하나를 상태에 반영한다.
func (m *Model) Apply(ev Event) {
	m.tally.add(ev)
	if ev.Kind == KindAllDone {
		m.done = true
		return
	}
	r := &m.rows[ev.Index]
	switch ev.Kind {
	case KindStart:
		*r = row{phase: rowRunning, started: m.now()}
	case KindStage:
		r.label = ev.Label
		if ev.Detail != "" {
			r.detail = ev.Detail
		}
	case KindDone:
		r.phase, r.outcome, r.label, r.detail = rowDone, ev.Outcome, ev.Label, ev.Detail
		r.elapsed, r.cost = ev.Elapsed, ev.CostUSD
	}
}

// Render 는 화면 문자열이다. 색은 기호에만 입혀 테스트에서 ANSI 를 벗기기 쉽게 한다.
func (m Model) Render() string {
	var b strings.Builder
	fmt.Fprintf(&b, " %s · %s\n\n", m.title, m.start.Format("15:04:05"))
	for i, it := range m.items {
		r := m.rows[i]
		var sym string
		var elapsed time.Duration
		label, detail, cost := r.label, r.detail, r.cost
		switch r.phase {
		case rowWaiting:
			sym, label, detail, cost = styleDim.Render("·"), "waiting", "", 0
		case rowRunning:
			sym, elapsed, cost = m.sp.View(), m.now().Sub(r.started), 0
		case rowDone:
			sym, elapsed = symbolStyle(r.outcome).Render(r.outcome.Symbol()), r.elapsed
		}
		b.WriteString(m.line(sym, it, detail, label, elapsed, cost) + "\n")
	}
	b.WriteString("\n " + strings.Repeat("─", 65) + "\n")
	summary := m.tally.summary(m.labels)
	if !m.done {
		summary += " so far"
	}
	b.WriteString(" " + summary + "\n")
	return b.String()
}

func symbolStyle(o Outcome) lipgloss.Style {
	switch o {
	case OutcomeOK:
		return styleOK
	case OutcomeFailed:
		return styleFailed
	default:
		return styleDim
	}
}

// line 은 한 항목의 줄이다: 기호 · 이름 · 부제 · [detail] · [label] · [경과] · [비용].
func (m Model) line(sym string, it Item, detail, label string, elapsed time.Duration, cost float64) string {
	var parts []string
	if detail != "" {
		parts = append(parts, fmt.Sprintf("%-12s", detail))
	}
	if label != "" {
		parts = append(parts, fmt.Sprintf("%-16s", label))
	}
	if e := elapsedText(elapsed); e != "" {
		parts = append(parts, fmt.Sprintf("%6s", e))
	}
	if c := costText(cost); c != "" {
		parts = append(parts, c)
	}
	s := fmt.Sprintf(" %s %-*s  %-*s  %s", sym, m.nameW, it.Name, m.subW, it.Sub, strings.Join(parts, "  "))
	return strings.TrimRight(s, " ")
}
```

`internal/tui/run.go`:

```go
package tui

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
)

// Options 는 Run 의 입력이다.
type Options struct {
	Title  string
	Items  []Item
	Labels Labels
	Out    io.Writer
	Log    io.Writer
	TTY    bool
	Now    func() time.Time
}

// ErrInterrupted 는 사용자가 ctrl+c 로 화면을 끝냈을 때다.
var ErrInterrupted = errors.New("interrupted by user")

// IsTerminal 은 f 가 문자 장치(터미널)인지 본다. 외부 모듈 없이 판별한다.
func IsTerminal(f *os.File) bool {
	st, err := f.Stat()
	return err == nil && st.Mode()&os.ModeCharDevice != 0
}

// Run 은 events 가 닫힐 때까지 소비한다. TTY 면 bubbletea 화면, 아니면 Out 에 한 줄씩. Log 에는 항상 한 줄씩 쓴다.
// ctrl+c 면 즉시 ErrInterrupted 를 돌려주고 남은 events 는 백그라운드에서 Log 로 흘린다 — 호출자는 ctx 를 취소하고
// 처리기가 끝나기를 기다린 뒤 Log 를 닫아야 한다.
func Run(ctx context.Context, o Options, events <-chan Event) error {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	var logPlain *Plain
	if o.Log != nil {
		logPlain = &Plain{W: o.Log, Items: o.Items, Labels: o.Labels, Now: now}
	}
	if !o.TTY {
		out := &Plain{W: o.Out, Items: o.Items, Labels: o.Labels, Now: now}
		for ev := range events {
			out.Write(ev)
			if logPlain != nil {
				logPlain.Write(ev)
			}
		}
		return nil
	}

	p := tea.NewProgram(NewModel(o.Title, o.Items, o.Labels, now), tea.WithContext(ctx), tea.WithOutput(o.Out))
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		for ev := range events {
			if logPlain != nil {
				logPlain.Write(ev)
			}
			p.Send(EventMsg(ev)) // 프로그램이 이미 끝났으면 no-op
		}
	}()
	final, err := p.Run()
	if errors.Is(err, tea.ErrInterrupted) {
		return ErrInterrupted
	}
	if err != nil {
		return err
	}
	if m, ok := final.(Model); ok && !m.Done() {
		return ErrInterrupted
	}
	<-drained
	return nil
}
```

- [x] **Step 4: 통과 확인**

Run: `go test ./internal/tui/ && go vet ./... && ls internal/tui/*.go | grep -v _test | xargs grep -n -i -E 'repo|graph|understand'; echo "grep exit=$?"`
Expected: PASS, `grep exit=1`.

- [x] **Step 5: 수동 확인 — 실제 터미널에서 스피너와 마지막 화면**

스크래치에 임시 main 을 만들어 돌려본다 (레포에 넣지 않는다):

```bash
S=/private/tmp/claude-501/-Users-jaeyoungcho-lab-gofer/30f5b3fa-4072-497d-a3f4-93ddd59b9d16/scratchpad/tuidemo
mkdir -p "$S" && cat > "$S/main.go" <<'EOF'
package main

import (
	"context"
	"os"
	"time"

	"gofer/internal/tui"
)

func main() {
	items := []tui.Item{{"example-api", "main"}, {"example-web", "develop"}, {"example-docs", "main"}}
	labels := tui.Labels{OK: "updated", Noop: "up to date", Skipped: "skipped", Failed: "failed"}
	events := make(chan tui.Event)
	go func() {
		events <- tui.Event{Kind: tui.KindStart, Index: 0}
		events <- tui.Event{Kind: tui.KindStage, Index: 0, Label: "fetch"}
		time.Sleep(1500 * time.Millisecond)
		events <- tui.Event{Kind: tui.KindStage, Index: 0, Label: "/understand …", Detail: "+13 commits"}
		time.Sleep(2500 * time.Millisecond)
		events <- tui.Event{Kind: tui.KindDone, Index: 0, Outcome: tui.OutcomeOK, Label: "graph updated", Detail: "+13 commits", Elapsed: 4 * time.Second, CostUSD: 1.83}
		events <- tui.Event{Kind: tui.KindStart, Index: 1}
		time.Sleep(time.Second)
		events <- tui.Event{Kind: tui.KindDone, Index: 1, Outcome: tui.OutcomeNoop, Label: "up to date"}
		events <- tui.Event{Kind: tui.KindStart, Index: 2}
		time.Sleep(time.Second)
		events <- tui.Event{Kind: tui.KindDone, Index: 2, Outcome: tui.OutcomeFailed, Label: "ff-merge failed: 1 local commit ahead of origin", Elapsed: time.Second}
		events <- tui.Event{Kind: tui.KindAllDone}
		close(events)
	}()
	err := tui.Run(context.Background(), tui.Options{Title: "ua-refresh · 3 repos", Items: items, Labels: labels, Out: os.Stdout, Log: os.Stderr, TTY: tui.IsTerminal(os.Stdout)}, events)
	if err != nil {
		os.Exit(1)
	}
}
EOF
mkdir -p ./cmd/_tuidemo && cp "$S/main.go" ./cmd/_tuidemo/main.go && go run ./cmd/_tuidemo 2>/dev/null; rm -rf ./cmd/_tuidemo
```

Expected: 처리 중인 줄에 스피너와 경과 시간이 움직이고, 끝난 뒤 세 줄과 요약(`1 updated · 1 up to date · 0 skipped · 1 failed · $1.83`)이 터미널에 남는다. `| cat` 을 붙여 실행하면 같은 내용이 한 줄 로그로 나온다. (`cmd/_tuidemo` 는 `_` 접두어라 `go build ./...` 가 무시하지만, 확인 후 반드시 지운다.)

- [x] **Step 6: 커밋**

```bash
git status --short   # cmd/_tuidemo 가 없어야 한다
git add internal/tui/ go.mod go.sum
git commit -m "feat(tui): bubbletea progress screen with plain-log fallback"
```

- [x] **Step 7: `/demiurge:rl` 로 완료조건 검증**

---

### Task 12: `run` 커맨드 — 설정·잠금·로그·화면·DM·last-run·종료 코드, `--dry-run`, `--only`

**Files:**
- Create: `internal/uarefresh/command.go`, `internal/uarefresh/command_test.go`, `cmd/uarefresh/run.go`
- Modify: `cmd/uarefresh/uarefresh.go` (등록 한 줄), `main.go` (시그널 → ctx 취소)

**Interfaces:**
- Consumes: Task 3 `Load`/`Paths`/`ResolveExtraPath`, Task 4 `Head`, Task 5 `DecideGraph`, Task 6 `lookPath`/`pathEnv`, Task 8 `AcquireLock`/`WriteRunResult`/`SlackText`/`Labels`, Task 9 `Run`/`GuardReason`, Task 10 `slack.Post`/`MacNotify`, Task 11 `tui.Run`/`tui.IsTerminal`/`tui.ErrInterrupted`.
- Produces:

```go
type CommandOptions struct {
	Paths  Paths
	DryRun bool
	Only   string
	Stdout io.Writer
	TTY    bool
	Now    func() time.Time
}
var ErrIncomplete = errors.New("some repositories were skipped or failed") // 종료 코드 1 용
func RunCommand(ctx context.Context, o CommandOptions) error
```

**스킬**: `superpowers:test-driven-development`. 완료 후 executing-plans 로 실행 중이면 `/code-review` 1회.

**완료조건**: `go test ./internal/uarefresh/ -run 'Command|DryRun'` PASS — 끝까지 실행(DM·last-run·로그·잠금 해제) · skipped 면 `ErrIncomplete` 이지만 DM 은 감 · Slack 실패 시 osascript · `--only` 필터와 없는 이름 오류 · dry-run 은 git/claude/파일을 건드리지 않음 · 잠금 중이면 `ErrAlreadyRunning`. `just build && ./bin/gofer ua-refresh run --help` 에 `--dry-run`, `--only` 가 보인다.

- [x] **Step 1: 실패하는 테스트**

`internal/uarefresh/command_test.go`:

```go
package uarefresh

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
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
	o, _ := setupCommand(t, f, slack.srv.URL, RepoConfig{Path: work, Trunk: "main"})

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
```

- [x] **Step 2: 실패 확인**

Run: `go test ./internal/uarefresh/ -run 'Command|DryRun'`
Expected: 컴파일 실패 (`undefined: RunCommand`).

- [x] **Step 3: 구현**

`internal/uarefresh/command.go`:

```go
package uarefresh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"gofer/internal/slack"
	"gofer/internal/tui"
)

// CommandOptions 는 `gofer ua-refresh run` 한 번의 입력이다.
type CommandOptions struct {
	Paths  Paths
	DryRun bool
	Only   string
	Stdout io.Writer
	TTY    bool
	Now    func() time.Time
}

// ErrIncomplete 는 skipped 나 failed 가 있어 종료 코드 1 이어야 할 때다 (설계 4절).
var ErrIncomplete = errors.New("some repositories were skipped or failed")

// RunCommand 는 설정 → (dry-run) → 잠금 → 로그 → 처리 + 화면 → last-run.json → Slack DM → 잠금 해제 순서다 (설계 4절).
func RunCommand(ctx context.Context, o CommandOptions) error {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	cfg, err := Load(o.Paths.Config)
	if err != nil {
		return err
	}
	repos, err := selectRepos(cfg.Repos, o.Only)
	if err != nil {
		return err
	}
	if o.DryRun {
		return dryRun(ctx, o.Stdout, cfg, repos)
	}

	release, err := AcquireLock(o.Paths.Lock())
	if err != nil {
		return err
	}
	defer release()

	started := now()
	logPath := o.Paths.DailyLog(started)
	logFile, err := openAppend(logPath)
	if err != nil {
		return err
	}
	defer logFile.Close()
	logw := &syncWriter{w: logFile}
	fmt.Fprintf(logw, "=== ua-refresh run %s · %d repos ===\n", started.Format(time.RFC3339), len(repos))

	items := make([]tui.Item, len(repos))
	for i, r := range repos {
		items[i] = tui.Item{Name: r.Name(), Sub: r.Trunk}
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	events := make(chan tui.Event, 16)
	resultCh := make(chan RunResult, 1)
	go func() {
		resultCh <- Run(ctx, RunParams{
			Repos: repos, Claude: cfg.Claude, ExtraPath: ResolveExtraPath(cfg.Env.ExtraPath),
			Log: logw, Events: events, Now: now,
		})
	}()
	uiErr := tui.Run(ctx, tui.Options{
		Title: fmt.Sprintf("ua-refresh · %d repos", len(repos)), Items: items, Labels: Labels,
		Out: o.Stdout, Log: logw, TTY: o.TTY, Now: now,
	}, events)
	if uiErr != nil {
		cancel() // ctrl+c: 진행 중인 claude 를 프로세스 그룹째 끝내고 결과를 기다린다
	}
	res := <-resultCh
	res.LogPath = logPath

	if err := WriteRunResult(o.Paths.LastRun(), res); err != nil {
		fmt.Fprintf(logw, "write last-run.json: %v\n", err)
	}
	// DM 은 ctx 가 취소됐어도 보낸다 (매 실행 1건, 설계 6절).
	if err := slack.Post(context.Background(), cfg.Notify.Slack.WebhookURL, SlackText(res)); err != nil {
		fmt.Fprintf(logw, "slack: %v\n", err)
		if nerr := MacNotify(context.Background(), "ua-refresh", res.Counts().String()+" (Slack failed, see log)"); nerr != nil {
			fmt.Fprintf(logw, "osascript: %v\n", nerr)
		}
	}
	if uiErr != nil {
		return uiErr
	}
	if res.ExitCode() != 0 {
		return ErrIncomplete
	}
	return nil
}

// selectRepos 는 --only 를 적용한다.
func selectRepos(all []RepoConfig, only string) ([]RepoConfig, error) {
	if only == "" {
		return all, nil
	}
	for _, r := range all {
		if r.Name() == only {
			return []RepoConfig{r}, nil
		}
	}
	return nil, fmt.Errorf("no repo named %q in config", only)
}

// dryRun 은 git 과 Claude 를 건드리지 않고 설정·가드·그래프 상태만 보고 무엇을 할지 출력한다 (설계 4절).
func dryRun(ctx context.Context, w io.Writer, cfg *Config, repos []RepoConfig) error {
	fmt.Fprintf(w, "config OK · %d repos · budget $%s/repo · timeout %s\n", len(repos),
		strconv.FormatFloat(cfg.Claude.BudgetUSD, 'f', -1, 64), cfg.Claude.Timeout())
	if bin, err := lookPath("claude", pathEnv(ResolveExtraPath(cfg.Env.ExtraPath))); err != nil {
		fmt.Fprintf(w, "warning: %v\n", err)
	} else {
		fmt.Fprintf(w, "claude: %s\n", bin)
	}
	nw, tw := 0, 0
	for _, r := range repos {
		nw, tw = max(nw, len(r.Name())), max(tw, len(r.Trunk))
	}
	for _, r := range repos {
		fmt.Fprintf(w, "%-*s  %-*s  %s\n", nw, r.Name(), tw, r.Trunk, dryRunPlan(ctx, r))
	}
	return nil
}

func dryRunPlan(ctx context.Context, r RepoConfig) string {
	reason, err := GuardReason(ctx, r)
	if err != nil {
		return "error: " + err.Error()
	}
	if reason != "" {
		return "skip: " + reason
	}
	head, err := Head(ctx, r.Path)
	if err != nil {
		return "error: " + err.Error()
	}
	d, hash, err := DecideGraph(ctx, r.Path, head)
	if err != nil {
		return "error: " + err.Error()
	}
	switch {
	case d == GraphUpToDate:
		return fmt.Sprintf("would fetch + ff-merge; graph at HEAD %.7s → /understand only if new commits arrive", head)
	case d == GraphFull:
		return fmt.Sprintf("would fetch + ff-merge, then /understand --full (graph commit %.7s not in repo)", hash)
	case hash == "":
		return "would fetch + ff-merge, then /understand (no graph yet)"
	default:
		return fmt.Sprintf("would fetch + ff-merge, then /understand (graph %.7s behind HEAD %.7s)", hash, head)
	}
}

func openAppend(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
}

// syncWriter 는 처리기와 화면 소비자가 같은 로그 파일에 동시에 쓸 때 줄이 섞이지 않게 한다.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *syncWriter) Write(b []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(b)
}
```

`cmd/uarefresh/run.go`:

```go
package uarefresh

import (
	"os"

	"github.com/spf13/cobra"

	"gofer/internal/tui"
	ua "gofer/internal/uarefresh"
)

func runCmd() *cobra.Command {
	var dryRun bool
	var only string
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Fetch, ff-merge and refresh the knowledge graph of every configured repo",
		Long: `Processes the repos in ~/.config/gofer/ua-refresh.toml in order: guard (root worktree on trunk,
no uncommitted tracked changes) → git fetch -ptf → git merge --ff-only → run /understand only when
the graph hash differs from HEAD. Sends one Slack DM at the end. Exit code 1 if any repo was skipped or failed.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			paths, err := ua.DefaultPaths()
			if err != nil {
				return err
			}
			return ua.RunCommand(cmd.Context(), ua.CommandOptions{
				Paths: paths, DryRun: dryRun, Only: only,
				Stdout: os.Stdout, TTY: tui.IsTerminal(os.Stdout),
			})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate config and print what would happen; touches neither git nor claude")
	cmd.Flags().StringVar(&only, "only", "", "process only the repo with this directory name")
	return cmd
}
```

`cmd/uarefresh/uarefresh.go` 의 `Cmd()` 에 추가:

```go
	cmd.AddCommand(runCmd())
```

`main.go` — SIGINT/SIGTERM 이 ctx 를 취소하게 한다. 이래야 launchd 의 `bootout` 이나 터미널의 ctrl+c 가 claude 프로세스 그룹 종료로 이어진다(자식은 별도 그룹이라 터미널 SIGINT 를 직접 받지 못한다):

```go
package main

import (
	"context"
	"os"
	"syscall"

	"github.com/charmbracelet/fang"

	"gofer/cmd"
)

// version 은 `just build` 가 -ldflags 로 채운다.
var version = "dev"

func main() {
	err := fang.Execute(context.Background(), cmd.Root(),
		fang.WithVersion(version),
		fang.WithNotifySignal(os.Interrupt, syscall.SIGTERM),
	)
	if err != nil {
		os.Exit(1)
	}
}
```

- [x] **Step 4: 통과 확인**

Run: `go test ./... && go vet ./... && just build && ./bin/gofer ua-refresh run --help | grep -E -- '--dry-run|--only'`
Expected: 전부 PASS, 두 플래그 표시.

- [x] **Step 5: 커밋**

```bash
git add internal/uarefresh/command.go internal/uarefresh/command_test.go cmd/uarefresh/run.go cmd/uarefresh/uarefresh.go main.go
git commit -m "feat(ua-refresh): run command with dry-run, lock, daily log, slack report"
```

- [x] **Step 6: `/demiurge:rl` 로 완료조건 검증** (executing-plans 로 실행 중이면 이어서 `/code-review`)

---

### Task 13: launchd 등록 — plist 생성, `install` / `uninstall`

**Files:**
- Create: `internal/uarefresh/launchd.go`, `internal/uarefresh/launchd_test.go`, `cmd/uarefresh/install.go`
- Modify: `cmd/uarefresh/uarefresh.go` (등록 두 줄)

**Interfaces:**
- Consumes: Task 3 `Paths`, `LaunchdLabel`, `Load`.
- Produces:

```go
type PlistParams struct {
	Label   string
	Program string   // gofer 실행 파일 절대 경로
	Args    []string // {"ua-refresh", "run"}
	Hour    int
	Minute  int
	LogPath string   // StandardOutPath 와 StandardErrorPath 둘 다
}
func PlistXML(p PlistParams) string
func ParseAt(at string) (hour, minute int, err error)                       // "07:30" → 7, 30
func Install(ctx context.Context, paths Paths, at, program string) error     // plist 쓰기 → bootout(무시) → bootstrap
func Uninstall(ctx context.Context, paths Paths) error                       // bootout → plist 삭제
func Installed(ctx context.Context) (bool, error)                            // launchctl print gui/<uid>/<label> 성공 여부
```

**스킬**: `superpowers:test-driven-development`.

**완료조건**: `go test ./internal/uarefresh/ -run 'Plist|ParseAt|Install|Uninstall|Installed'` PASS. `just build && ./bin/gofer ua-refresh install --help && ./bin/gofer ua-refresh uninstall --help` 종료 코드 0. **실제 등록은 Task 15 에서만 한다.**

- [x] **Step 1: 실패하는 테스트**

`internal/uarefresh/launchd_test.go`:

```go
package uarefresh

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// installFakeLaunchctl 은 인자를 한 줄씩 누적 기록하는 가짜 launchctl 을 PATH 앞에 둔다.
// FAKE_LAUNCHCTL_PRINT_EXIT 로 `print` 의 종료 코드를 정한다 (기본 0).
func installFakeLaunchctl(t *testing.T) (argsFile string) {
	t.Helper()
	dir := t.TempDir()
	argsFile = filepath.Join(dir, "args")
	script := "#!/bin/sh\necho \"$*\" >> \"" + argsFile + "\"\n" +
		"if [ \"$1\" = print ]; then exit \"${FAKE_LAUNCHCTL_PRINT_EXIT:-0}\"; fi\n"
	if err := os.WriteFile(filepath.Join(dir, "launchctl"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argsFile
}

func TestPlistXML(t *testing.T) {
	got := PlistXML(PlistParams{
		Label: "gofer.ua-refresh", Program: "/Users/example/.local/bin/gofer", Args: []string{"ua-refresh", "run"},
		Hour: 7, Minute: 30, LogPath: "/Users/example/Library/Logs/gofer/ua-refresh/launchd.log",
	})
	want := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>gofer.ua-refresh</string>
	<key>ProgramArguments</key>
	<array>
		<string>/Users/example/.local/bin/gofer</string>
		<string>ua-refresh</string>
		<string>run</string>
	</array>
	<key>StartCalendarInterval</key>
	<dict>
		<key>Hour</key>
		<integer>7</integer>
		<key>Minute</key>
		<integer>30</integer>
	</dict>
	<key>StandardOutPath</key>
	<string>/Users/example/Library/Logs/gofer/ua-refresh/launchd.log</string>
	<key>StandardErrorPath</key>
	<string>/Users/example/Library/Logs/gofer/ua-refresh/launchd.log</string>
</dict>
</plist>
`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPlistXMLEscapes(t *testing.T) {
	got := PlistXML(PlistParams{Label: "x", Program: "/tmp/a&b/gofer", Args: []string{"<run>"}, LogPath: "/l"})
	if !strings.Contains(got, "/tmp/a&amp;b/gofer") || !strings.Contains(got, "&lt;run&gt;") {
		t.Errorf("not escaped:\n%s", got)
	}
}

func TestParseAt(t *testing.T) {
	if h, m, err := ParseAt("07:30"); err != nil || h != 7 || m != 30 {
		t.Errorf("got %d:%d %v", h, m, err)
	}
	if _, _, err := ParseAt("7:30pm"); err == nil {
		t.Error("want error for bad format")
	}
}

func TestInstallWritesPlistAndBootstraps(t *testing.T) {
	argsFile := installFakeLaunchctl(t)
	base := t.TempDir()
	paths := Paths{LogDir: filepath.Join(base, "logs"), Plist: filepath.Join(base, "LaunchAgents", "gofer.ua-refresh.plist")}
	if err := Install(t.Context(), paths, "07:30", "/usr/local/bin/gofer"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(paths.Plist)
	if err != nil || !strings.Contains(string(b), "<string>/usr/local/bin/gofer</string>") || !strings.Contains(string(b), "<integer>30</integer>") {
		t.Errorf("plist: %v\n%s", err, b)
	}
	if st, _ := os.Stat(paths.LogDir); st == nil || !st.IsDir() {
		t.Error("log dir should be created so launchd can open the log")
	}
	calls, _ := os.ReadFile(argsFile)
	want := fmt.Sprintf("bootout gui/%d/gofer.ua-refresh\nbootstrap gui/%d %s\n", os.Getuid(), os.Getuid(), paths.Plist)
	if string(calls) != want {
		t.Errorf("launchctl calls:\n%s\nwant:\n%s", calls, want)
	}
}

func TestUninstallBootsOutAndRemovesPlist(t *testing.T) {
	argsFile := installFakeLaunchctl(t)
	paths := Paths{Plist: filepath.Join(t.TempDir(), "gofer.ua-refresh.plist")}
	os.WriteFile(paths.Plist, []byte("x"), 0o644)
	if err := Uninstall(t.Context(), paths); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(paths.Plist); err == nil {
		t.Error("plist should be removed")
	}
	calls, _ := os.ReadFile(argsFile)
	if want := fmt.Sprintf("bootout gui/%d/gofer.ua-refresh\n", os.Getuid()); string(calls) != want {
		t.Errorf("launchctl calls:\n%s\nwant:\n%s", calls, want)
	}
}

func TestInstalled(t *testing.T) {
	installFakeLaunchctl(t)
	if ok, err := Installed(t.Context()); err != nil || !ok {
		t.Errorf("print exit 0 → installed; got %v %v", ok, err)
	}
	t.Setenv("FAKE_LAUNCHCTL_PRINT_EXIT", "113")
	if ok, err := Installed(t.Context()); err != nil || ok {
		t.Errorf("print exit 113 → not installed; got %v %v", ok, err)
	}
}
```

- [x] **Step 2: 실패 확인**

Run: `go test ./internal/uarefresh/ -run 'Plist|ParseAt|Install|Uninstall|Installed'`
Expected: 컴파일 실패.

- [x] **Step 3: 구현**

`internal/uarefresh/launchd.go`:

```go
package uarefresh

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// PlistParams 는 LaunchAgent plist 에 들어가는 값이다 (설계 3절 파일 위치, 2절 launchd 근거).
type PlistParams struct {
	Label   string
	Program string
	Args    []string
	Hour    int
	Minute  int
	LogPath string
}

// PlistXML 은 StartCalendarInterval 로 매일 한 번 실행되는 plist 본문이다. 잠든 동안 놓친 예약은 wake 시 실행된다.
func PlistXML(p PlistParams) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>` + xmlEscape(p.Label) + `</string>
	<key>ProgramArguments</key>
	<array>
		<string>` + xmlEscape(p.Program) + `</string>
`)
	for _, a := range p.Args {
		b.WriteString("\t\t<string>" + xmlEscape(a) + "</string>\n")
	}
	fmt.Fprintf(&b, `	</array>
	<key>StartCalendarInterval</key>
	<dict>
		<key>Hour</key>
		<integer>%d</integer>
		<key>Minute</key>
		<integer>%d</integer>
	</dict>
	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
</dict>
</plist>
`, p.Hour, p.Minute, xmlEscape(p.LogPath), xmlEscape(p.LogPath))
	return b.String()
}

func xmlEscape(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

// ParseAt 은 설정의 schedule.at ("HH:MM") 을 시·분으로 푼다.
func ParseAt(at string) (int, int, error) {
	t, err := time.Parse("15:04", at)
	if err != nil {
		return 0, 0, fmt.Errorf("schedule.at must be HH:MM, got %q", at)
	}
	return t.Hour(), t.Minute(), nil
}

func domainTarget() string { return fmt.Sprintf("gui/%d", os.Getuid()) }

func launchctl(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "launchctl", args...).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("launchctl %s: %w: %s", strings.Join(args, " "), err, bytes.TrimSpace(out))
	}
	return string(out), nil
}

// Install 은 plist 를 쓰고 launchd 에 올린다. 이미 올라가 있으면 내렸다가 다시 올려 시각 변경을 반영한다.
func Install(ctx context.Context, paths Paths, at, program string) error {
	hour, minute, err := ParseAt(at)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(paths.LogDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(paths.Plist), 0o755); err != nil {
		return err
	}
	xmlBody := PlistXML(PlistParams{
		Label: LaunchdLabel, Program: program, Args: []string{"ua-refresh", "run"},
		Hour: hour, Minute: minute, LogPath: paths.LaunchdLog(),
	})
	if err := os.WriteFile(paths.Plist, []byte(xmlBody), 0o644); err != nil {
		return err
	}
	_, _ = launchctl(ctx, "bootout", domainTarget()+"/"+LaunchdLabel) // 안 올라가 있으면 실패하는 게 정상
	_, err = launchctl(ctx, "bootstrap", domainTarget(), paths.Plist)
	return err
}

// Uninstall 은 launchd 에서 내리고 plist 를 지운다.
func Uninstall(ctx context.Context, paths Paths) error {
	if _, err := launchctl(ctx, "bootout", domainTarget()+"/"+LaunchdLabel); err != nil {
		if _, statErr := os.Stat(paths.Plist); statErr == nil {
			return err // plist 는 있는데 못 내렸다 — 진짜 오류
		}
	}
	if err := os.Remove(paths.Plist); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Installed 는 launchd 에 작업이 올라가 있는지 본다.
func Installed(ctx context.Context) (bool, error) {
	cmd := exec.CommandContext(ctx, "launchctl", "print", domainTarget()+"/"+LaunchdLabel)
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
```

`cmd/uarefresh/install.go`:

```go
package uarefresh

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	ua "gofer/internal/uarefresh"
)

func installCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Register the daily launchd job (uses schedule.at from the config)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			paths, err := ua.DefaultPaths()
			if err != nil {
				return err
			}
			cfg, err := ua.Load(paths.Config)
			if err != nil {
				return err
			}
			exe, err := os.Executable()
			if err != nil {
				return err
			}
			if resolved, err := filepath.EvalSymlinks(exe); err == nil {
				exe = resolved
			}
			if err := ua.Install(cmd.Context(), paths, cfg.Schedule.At, exe); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "installed %s: daily at %s\n  program: %s\n  plist:   %s\nre-run install if you move the gofer binary or change schedule.at\n",
				ua.LaunchdLabel, cfg.Schedule.At, exe, paths.Plist)
			return nil
		},
	}
}

func uninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the launchd job",
		RunE: func(cmd *cobra.Command, _ []string) error {
			paths, err := ua.DefaultPaths()
			if err != nil {
				return err
			}
			if err := ua.Uninstall(cmd.Context(), paths); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", ua.LaunchdLabel)
			return nil
		},
	}
}
```

`cmd/uarefresh/uarefresh.go` 의 `Cmd()` 에 추가:

```go
	cmd.AddCommand(installCmd(), uninstallCmd())
```

- [x] **Step 4: 통과 확인**

Run: `go test ./... && go vet ./... && just build && ./bin/gofer ua-refresh install --help >/dev/null && ./bin/gofer ua-refresh uninstall --help >/dev/null && echo OK`
Expected: PASS, `OK`.

- [x] **Step 5: 커밋**

```bash
git add internal/uarefresh/launchd.go internal/uarefresh/launchd_test.go cmd/uarefresh/install.go cmd/uarefresh/uarefresh.go
git commit -m "feat(ua-refresh): launchd install/uninstall with generated plist"
```

- [x] **Step 6: `/demiurge:rl` 로 완료조건 검증**

---

### Task 14: `status` · `log` 커맨드

**Files:**
- Create: `internal/uarefresh/status.go`, `internal/uarefresh/status_test.go`, `internal/uarefresh/logcmd.go`, `internal/uarefresh/logcmd_test.go`, `cmd/uarefresh/status.go`, `cmd/uarefresh/log.go`
- Modify: `cmd/uarefresh/uarefresh.go` (등록 두 줄)

**Interfaces:**
- Consumes: Task 3 `Paths`/`Load`, Task 4 `Head`, Task 5 `GraphHash`/`ErrNoGraph`, Task 8 `ReadRunResult`/`shortenHome`, Task 13 `Installed`.
- Produces:

```go
func StatusText(ctx context.Context, paths Paths, cfg *Config, installed bool) string
func ShowLog(ctx context.Context, path string, w io.Writer, follow bool) error
```

**스킬**: `superpowers:test-driven-development`.

**완료조건**: `go test ./internal/uarefresh/ -run 'Status|ShowLog'` PASS. `just build && ./bin/gofer ua-refresh status` 가 (설정이 유효하면) 세 구획(launchd · last run · repos)을 출력하고, `./bin/gofer ua-refresh log` 가 오늘 로그 또는 `no log yet` 을 출력한다.

- [x] **Step 1: 실패하는 테스트**

`internal/uarefresh/status_test.go`:

```go
package uarefresh

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestStatusText(t *testing.T) {
	fresh, _ := newRepoWithOrigin(t, "main")
	freshHead, _ := Head(t.Context(), fresh)
	writeMeta(t, fresh, ".ua", freshHead)
	stale, _ := newRepoWithOrigin(t, "develop")
	writeMeta(t, stale, ".ua", "0123456789abcdef0123456789abcdef01234567")
	none, _ := newRepoWithOrigin(t, "main")

	base := t.TempDir()
	paths := Paths{StateDir: filepath.Join(base, "state"), Plist: filepath.Join(base, "gofer.ua-refresh.plist")}
	if err := WriteRunResult(paths.LastRun(), sampleRun()); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Schedule: ScheduleConfig{At: "07:30"}, Repos: []RepoConfig{{Path: fresh, Trunk: "main"}, {Path: stale, Trunk: "develop"}, {Path: none, Trunk: "main"}}}

	got := StatusText(t.Context(), paths, cfg, true)
	for _, want := range []string{
		"launchd: installed · daily at 07:30",
		"last run: 2026-09-05 08:30 → 08:41 · 2 updated · 1 up to date · 0 skipped · 1 failed · $2.54 · exit 1",
		"HEAD " + freshHead[:7] + "  graph " + freshHead[:7] + "  fresh",
		"graph 0123456  stale",
		"graph -        none",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}

	got = StatusText(t.Context(), Paths{StateDir: t.TempDir()}, cfg, false)
	if !strings.Contains(got, "launchd: not installed") || !strings.Contains(got, "last run: never") {
		t.Errorf("got:\n%s", got)
	}
}
```

`internal/uarefresh/logcmd_test.go`:

```go
package uarefresh

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestShowLogPrintsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "2026-09-05.log")
	os.WriteFile(path, []byte("a\nb\n"), 0o644)
	var out bytes.Buffer
	if err := ShowLog(t.Context(), path, &out, false); err != nil || out.String() != "a\nb\n" {
		t.Fatalf("got %q %v", out.String(), err)
	}
}

func TestShowLogMissingFile(t *testing.T) {
	var out bytes.Buffer
	if err := ShowLog(t.Context(), filepath.Join(t.TempDir(), "none.log"), &out, false); err != nil || !strings.HasPrefix(out.String(), "no log yet:") {
		t.Fatalf("got %q %v", out.String(), err)
	}
}

func TestShowLogFollowAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "2026-09-05.log")
	os.WriteFile(path, []byte("a\n"), 0o644)
	ctx, cancel := context.WithCancel(t.Context())
	var mu sync.Mutex
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() { done <- ShowLog(ctx, path, lockedWriter{&mu, &out}, true) }()
	time.Sleep(200 * time.Millisecond)
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString("b\n")
	f.Close()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		s := out.String()
		mu.Unlock()
		if s == "a\nb\n" {
			cancel()
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	cancel()
	t.Fatalf("follow did not pick up the appended line; got %q", out.String())
}

type lockedWriter struct {
	mu *sync.Mutex
	w  *bytes.Buffer
}

func (l lockedWriter) Write(b []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(b)
}
```

- [x] **Step 2: 실패 확인**

Run: `go test ./internal/uarefresh/ -run 'Status|ShowLog'`
Expected: 컴파일 실패.

- [x] **Step 3: 구현**

`internal/uarefresh/status.go`:

```go
package uarefresh

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// StatusText 는 `gofer ua-refresh status` 본문이다: launchd 등록 여부 · 마지막 실행 · 레포별 그래프 신선도 (설계 3절).
func StatusText(ctx context.Context, paths Paths, cfg *Config, installed bool) string {
	var b strings.Builder
	if installed {
		fmt.Fprintf(&b, "launchd: installed · daily at %s · %s\n", cfg.Schedule.At, shortenHome(paths.Plist))
	} else {
		b.WriteString("launchd: not installed (run `gofer ua-refresh install`)\n")
	}

	if last, err := ReadRunResult(paths.LastRun()); err == nil {
		fmt.Fprintf(&b, "last run: %s → %s · %s · exit %d\n          log: %s\n",
			last.StartedAt.Format("2006-01-02 15:04"), last.FinishedAt.Format("15:04"), last.Counts(), last.ExitCode(), shortenHome(last.LogPath))
	} else {
		b.WriteString("last run: never\n")
	}

	b.WriteString("repos:\n")
	nw, tw := 0, 0
	for _, r := range cfg.Repos {
		nw, tw = max(nw, len(r.Name())), max(tw, len(r.Trunk))
	}
	for _, r := range cfg.Repos {
		fmt.Fprintf(&b, "  %-*s  %-*s  %s\n", nw, r.Name(), tw, r.Trunk, graphFreshness(ctx, r.Path))
	}
	return b.String()
}

// graphFreshness 는 "HEAD abcdef0  graph abcdef0  fresh|stale|none" 이다.
func graphFreshness(ctx context.Context, repo string) string {
	head, err := Head(ctx, repo)
	if err != nil {
		return "error: " + err.Error()
	}
	hash, err := GraphHash(repo)
	switch {
	case errors.Is(err, ErrNoGraph):
		return fmt.Sprintf("HEAD %.7s  graph %-7s  none", head, "-")
	case err != nil:
		return fmt.Sprintf("HEAD %.7s  graph error: %v", head, err)
	case hash == head:
		return fmt.Sprintf("HEAD %.7s  graph %.7s  fresh", head, hash)
	default:
		return fmt.Sprintf("HEAD %.7s  graph %.7s  stale", head, hash)
	}
}
```

`internal/uarefresh/logcmd.go`:

```go
package uarefresh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// ShowLog 는 로그 파일을 w 에 쓴다. follow 면 ctx 가 끝날 때까지 새로 붙는 내용을 계속 쓴다 (파일이 나중에 생겨도 된다).
func ShowLog(ctx context.Context, path string, w io.Writer, follow bool) error {
	var offset int64
	copyNew := func() error {
		f, err := os.Open(path)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return err
		}
		n, err := io.Copy(w, f)
		offset += n
		return err
	}
	if err := copyNew(); err != nil {
		return err
	}
	if !follow {
		if offset == 0 {
			fmt.Fprintf(w, "no log yet: %s\n", path)
		}
		return nil
	}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := copyNew(); err != nil {
				return err
			}
		}
	}
}
```

`cmd/uarefresh/status.go`:

```go
package uarefresh

import (
	"fmt"

	"github.com/spf13/cobra"

	ua "gofer/internal/uarefresh"
)

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show launchd registration, last run summary and per-repo graph freshness",
		RunE: func(cmd *cobra.Command, _ []string) error {
			paths, err := ua.DefaultPaths()
			if err != nil {
				return err
			}
			cfg, err := ua.Load(paths.Config)
			if err != nil {
				return err
			}
			installed, err := ua.Installed(cmd.Context())
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), ua.StatusText(cmd.Context(), paths, cfg, installed))
			return nil
		},
	}
}
```

`cmd/uarefresh/log.go`:

```go
package uarefresh

import (
	"time"

	"github.com/spf13/cobra"

	ua "gofer/internal/uarefresh"
)

func logCmd() *cobra.Command {
	var follow bool
	cmd := &cobra.Command{
		Use:   "log",
		Short: "Print today's log (--follow to keep watching)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			paths, err := ua.DefaultPaths()
			if err != nil {
				return err
			}
			return ua.ShowLog(cmd.Context(), paths.DailyLog(time.Now()), cmd.OutOrStdout(), follow)
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "keep printing as the log grows (ctrl+c to stop)")
	return cmd
}
```

`cmd/uarefresh/uarefresh.go` 의 `Cmd()` 에 추가:

```go
	cmd.AddCommand(statusCmd(), logCmd())
```

- [x] **Step 4: 통과 확인**

Run: `go test ./... && go vet ./... && just build && ./bin/gofer ua-refresh log; ./bin/gofer ua-refresh status`
Expected: PASS. `log` 는 `no log yet: ...`. `status` 는 설정이 유효하면 세 구획을 출력하고, `webhook_url` 이 아직 비어 있으면 설정 오류를 출력한다(이것도 정상 — Task 15 에서 채운다).

- [x] **Step 5: 커밋**

```bash
git add internal/uarefresh/status.go internal/uarefresh/status_test.go internal/uarefresh/logcmd.go internal/uarefresh/logcmd_test.go cmd/uarefresh/status.go cmd/uarefresh/log.go cmd/uarefresh/uarefresh.go
git commit -m "feat(ua-refresh): status and log commands"
```

- [x] **Step 6: `/demiurge:rl` 로 완료조건 검증**

---

### Task 15: 통합 검증 (실제 레포·실제 Claude) 과 인계 문서 마감

**Files:**
- Modify: `docs/autopilot/ua-refresh/HANDOFF.md` (현황 갱신)
- 사용자 환경: `~/.local/bin/gofer`(바이너리 복사), `~/.config/gofer/ua-refresh.toml`(`webhook_url`), `~/Library/LaunchAgents/gofer.ua-refresh.plist`

**Interfaces:**
- Consumes: 완성된 바이너리 전체.
- Produces: 운영 상태. 새 사실은 HANDOFF.md 에 남긴다.

**스킬**: `superpowers:verification-before-completion`. 예상 밖 동작은 `/demiurge:debug`. 마지막에 `/demiurge:rl` 로 플랜 전체 완료조건 A~E 검증.

**완료조건**: 아래 Step 순서대로 전부 통과하고, 다음 날 아침 DM 이 도착했다. 플랜 전체 완료조건 표 A~E 충족.

- [x] **Step 1: 바이너리를 안정된 경로에 두고 설정을 마무리한다** (2026-09-04: ~/.local/bin/gofer 로 복사, webhook_url 사용자가 채움)

launchd plist 는 `install` 시점의 실행 파일 경로를 박아 두므로 레포 안의 `bin/gofer` 가 아니라 고정 위치에서 등록한다.

```bash
just build && cp bin/gofer ~/.local/bin/gofer && gofer --version
```

사용자가 Slack Incoming Webhook(대상: 본인 DM)을 발급해 `~/.config/gofer/ua-refresh.toml` 의 `webhook_url` 을 채운다. 이 값은 절대 레포·플랜·인계 문서에 적지 않는다.

- [x] **Step 2: `config init` 이 기존 파일을 보존하는지** (2026-09-04: "already exists" 오류, shasum OK)

```bash
shasum ~/.config/gofer/ua-refresh.toml > /tmp/before.sha
gofer ua-refresh config init; echo "exit=$?"
shasum -c /tmp/before.sha
```

Expected: `already exists` 오류, `exit=1`, `shasum -c` 가 `OK`.

- [x] **Step 3: dry-run** (2026-09-04: config OK · 6 repos, `error:` 없음; 1개 레포가 `skip:` — 루트 브랜치와 설정 trunk 불일치, 사용자 확인 필요)

```bash
gofer ua-refresh run --dry-run
```

Expected: `config OK · N repos`, `claude: /Users/.../.local/bin/claude`, 레포마다 한 줄. `error:` 줄이 없어야 한다. `skip:` 이 있으면 그 레포의 실제 상태(브랜치·미커밋)를 확인하고 정리한 뒤 다시 돌린다 — 도구가 아니라 레포 상태가 원인이다.

- [x] **Step 4: 작은 레포 하나로 실제 실행** (2026-09-04: 1차 — 스킬이 confirm 을 기다려 실패 → `--append-system-prompt` 수정; 2차 — 사용량 창 한도; 3차 — 중단 후 model=opus 로 전환; 4차 — ✓ graph updated 11m33s $5.61, HEAD==graph)

```bash
gofer ua-refresh run --only <가장 작은 레포 디렉토리명>; echo "exit=$?"
gofer ua-refresh status
```

Expected: 화면에 스피너 → `graph updated` 또는 `up to date`; Slack DM 1건 도착; `status` 의 그 레포가 `fresh`; `exit=0`. `up to date` 였다면 그래프가 이미 최신인 것이니 다른 레포로 한 번 더 해 실제 `/understand` 경로를 확인한다.

- [x] **Step 5: 전체 실행** (2026-09-04: 6개 레포 전부 fresh, 종료 코드 0 — 중간에 예산 초과·사용량 창 문제 발생 → model=sonnet, budget_usd=30 으로 조정 후 성공)

```bash
gofer ua-refresh run; echo "exit=$?"
gofer ua-refresh log | tail -30
```

Expected: 모든 레포가 updated/up-to-date, DM 1건, `exit=0`. skipped/failed 가 있으면 DM 과 로그의 사유를 보고 레포 상태를 정리하거나(가드) 버그면 `/demiurge:debug` 로 고친 뒤 재실행.

- [x] **Step 6: launchd 등록** (2026-09-04: installed gofer.ua-refresh: daily at 07:30, program=~/.local/bin/gofer, launchctl print 확인됨)

```bash
gofer ua-refresh install
launchctl print gui/$(id -u)/gofer.ua-refresh | head -20
gofer ua-refresh status | head -1
```

Expected: `installed gofer.ua-refresh: daily at 07:30`, `launchctl print` 에 `program = /Users/.../.local/bin/gofer`, `status` 첫 줄 `launchd: installed`.

- [x] **Step 7: 플랜 완료조건 C·D 검증** (2026-09-04: C — go list -m 정확히 6개; D — 레포 7종+trunk 명으로 검색, 결과 없음)

```bash
go list -m -f '{{if not .Indirect}}{{.Path}}{{end}}' all | grep -v '^gofer$' | sort   # 정확히 6줄
# 조직 중립: 설정 파일의 path·trunk 값에서 회사/레포/브랜치 이름을 뽑아 검색어로 쓴다 (값을 여기에 적지 않는다)
git grep -n -i -E '<검색어1>|<검색어2>' -- . ':!docs/superpowers/plans'                 # 결과 없음
```

- [x] **Step 8: 인계 문서 갱신 · 커밋** (2026-09-04: HANDOFF.md 최종 상태로 갱신, 이 커밋)

`docs/autopilot/ua-refresh/HANDOFF.md`:
- 2절 TL;DR: "코드 없음" → 구현 완료·launchd 등록됨·운영 관찰 대기 로 갱신.
- 4절 남은 작업: "다음 날 아침 DM 확인" 만 남긴다.
- 8절 환경 사실: Task 1 의 인증 결과, `gofer` 설치 경로(`~/.local/bin/gofer`), 실제 실행에서 배운 것(레포당 소요 시간·비용 범위 등 조직 정보가 아닌 것만).

```bash
git add docs/autopilot/ua-refresh/HANDOFF.md
git commit -m "docs: ua-refresh integration verified, launchd job installed"
```

- [ ] **Step 9: 다음 날 아침 운영 확인**

Expected: 설정 시각(또는 wake 직후)에 DM 도착. `gofer ua-refresh status` 의 `last run` 이 오늘 날짜. 도착하지 않았으면 `~/Library/Logs/gofer/ua-refresh/launchd.log` 와 오늘 로그를 본다 — 인증(Task 1 결과 재확인), PATH(`extra_path`), 잠금 파일 순으로 의심한다.

- [ ] **Step 10: `/demiurge:rl` 로 플랜 전체 완료조건 A~E 검증**

---

## Self-Review 기록 (플랜 작성자)

- **설계 커버리지**: 1절 목표(순차 처리·비용 0·상태 보호·DM 1통·TUI) → Task 9·12·11. 2절 전제·`/understand` 동작·헤드리스 사실 → Task 5·6·1. 3절 사용법 5개 커맨드 → Task 3·12·13·14, 설정 스키마·검증 → Task 3, 파일 위치 → Task 3 `Paths`. 4절 흐름 ①~⑥·결과 4종·종료 코드·dry-run → Task 9·8·12. 5절 코드 구조·의존성 → Task 2 + 파일 구조. 6절 화면·DM·로그 → Task 11·8·12. 7절 안전장치(잠금·설정 오류·가드·ff 실패·타임아웃·Slack 실패) → Task 8·3·9·6·12. 8절 테스트 단계 → 각 Task 의 테스트 + Task 1·15. 9절 결정 기록은 구현 지침으로 반영됨(worktree 미사용, trunk 명시, 해시 선비교).
- **설계와 다른 점 1건**: Charm v2 모듈 경로가 `github.com/charmbracelet/...` 가 아니라 `charm.land/...` 임을 모듈 프록시에서 확인하고 설계 5절 표를 고쳤다. 라이브러리 선택·버전은 그대로다.
- **타입 일관성**: `tui.Event{Kind, Index, Outcome, Label, Detail, Elapsed, CostUSD}`, `RepoResult{Name, Trunk, Status, Commits, Reason, Elapsed, CostUSD}`, `RunParams{Repos, Claude, ExtraPath, Log, Events, Now}`, `CommandOptions{Paths, DryRun, Only, Stdout, TTY, Now}`, `ClaudeOptions{Dir, Full, BudgetUSD, Timeout, Model, ExtraPath, OAuthToken, Stderr}` 를 정의 Task 와 사용 Task 에서 같은 이름으로 썼다. 헬퍼 `commitsText` 는 Task 8(report.go)에서 정의하고 Task 9 가 사용한다. `writeMeta` 는 Task 5 테스트에서 정의하고 Task 9·12·14 테스트가 사용한다. `installFakeOsascript` 는 Task 10 에서 정의하고 Task 12 가 사용한다.
