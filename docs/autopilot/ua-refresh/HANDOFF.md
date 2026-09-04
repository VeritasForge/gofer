# HANDOFF — gofer ua-refresh

## 1. ⚡ 즉시 재진입

`/superpowers:writing-plans` 를 호출해 `docs/superpowers/specs/2026-09-04-ua-refresh-design.md` 를 바탕으로 구현 플랜을 `docs/superpowers/plans/` 에 쓴다.

## 2. 📍 TL;DR — 어디서 멈췄나

- 브랜치: `main` (origin 없음, 새 레포)
- 마지막 커밋: `42f821f` — 설계 문서 + CLAUDE.md
- 코드: 아직 없음 (go.mod 도 없음). 테스트 베이스라인: 해당 없음
- 다음 단계: 구현 플랜 작성 → 플랜 실행. 브레인스토밍은 끝났고 설계는 사용자 승인됨
- 이 레포는 `~/workspace/tools`(빈 레포)에서 옮겨온 것. 원래 디렉토리는 손대지 않았음

## 3. ✅ 완료

| 작업 | 커밋 | 요약 |
|---|---|---|
| 브레인스토밍 · 설계 승인 | — | 대화로 진행, 결과는 설계 문서 9절 "결정 기록"에 정리 |
| 설계 문서 · 프로젝트 규칙 | `42f821f` | `docs/superpowers/specs/2026-09-04-ua-refresh-design.md`, `CLAUDE.md` |

## 4. ⏳ 남은 작업

| 작업 | 핵심 액션 | 예상 |
|---|---|---|
| 구현 플랜 작성 | writing-plans. 사용자 전역 CLAUDE.md 의 "플랜 작성 템플릿"(완료조건·스킬 검색·경로 검증·Task 별 검증) 준수 | 1 세션 |
| 인증 실험 (플랜 첫 Task 로 넣을 것) | launchd 에서 `claude -p "reply ok" --output-format json` 이 되는지. 안 되면 `claude setup-token` → 설정의 `oauth_token` | 짧음 |
| 구현 | 플랜대로. 모듈 초기화 → 설정 → git → Claude 실행 → 오케스트레이터/Event → tui → slack → launchd → cmd 배선 | 플랜이 정함 |
| 통합 검증 | `config init` 은 기존 파일 보존 → `run --dry-run` → `run --only <레포>` → `run` → `install` → 다음 날 DM 확인 | 1~2 세션 + 하룻밤 |

## 5. 🛡 누적 judgments

| 결정 | 적용 위치 |
|---|---|
| 조직 중립: 코드·주석·문서·예제에 회사/레포/브랜치/워크스페이스 정보 금지. 실제 값은 설정 파일에만 | `CLAUDE.md`, 설계 1·9절 |
| 실제 레포 목록·trunk 는 **이미** `~/.config/gofer/ua-refresh.toml` 에 채워둠(0600, git 밖). `webhook_url` 만 비어 있음 — 사용자가 Slack Incoming Webhook(대상: 본인 DM) 발급 후 채움 | 설정 파일 |
| 설정 스키마가 구현 중 바뀌면 그 파일도 같이 고칠 것. `config init` 은 기존 파일을 덮어쓰지 않는다 | 설계 3절 |
| 루트 작업 트리 = 항상 trunk (기능 작업은 worktree). 어긋나면 그 레포 skip | 설계 2·4절 |
| trunk 는 설정에 명시, origin/HEAD 로 추론 금지 | 설계 2절 |
| Claude 호출 전에 그래프 해시 == HEAD 비교, 같으면 호출 안 함 (`/understand` 가 되묻고 멈추기 때문) | 설계 2·4절 |
| 그래프 커밋이 레포에 없으면 `--full` | 설계 4절 |
| worktree 에서 `/understand` 돌리는 방식은 성립 안 함 (리다이렉트가 분석 루트 자체를 바꿈) | 설계 9절 |
| `--bare` 금지, `--dangerously-skip-permissions` 필요, `--max-budget-usd` 사용 | 설계 2절 |
| Go 1.26 · cobra + fang · bubbletea/v2 · bubbles/v2 · lipgloss/v2 · BurntSushi/toml. 버전은 설계 5절 표 | 설계 5절 |
| 바이너리 하나 + `gofer <도구> <동작>`. `cmd/` 얇게, `internal/` 에 로직, Event 기반 화면 분리 | 설계 5절 |
| 성공 판정은 Claude 종료 코드가 아니라 그래프 해시 == HEAD | 설계 4절 |
| 타임아웃은 프로세스 그룹 단위 종료 (macOS 에 `timeout` 명령 없음) | 설계 4절 |
| launchd (cron 아님), 잠들면 wake 시 실행. 자동 기상은 이번 범위 밖 | 설계 1·2절 |
| Slack DM 은 매 실행 종료 시 항상 1건. 실패 시 osascript | 설계 6절 |

## 6. 🚨 외부 review 패턴

해당 없음 (아직 코드 없음). 코드 Task 완료 후 리뷰는 전역 CLAUDE.md 규칙(내장 리뷰 없으면 `/code-review` 1회)을 따른다.

## 7. 🗂 핵심 파일 위치

| 무엇 | 경로 |
|---|---|
| 설계 문서 | `docs/superpowers/specs/2026-09-04-ua-refresh-design.md` |
| 프로젝트 규칙 | `CLAUDE.md` |
| 실제 사용자 설정 (git 밖) | `~/.config/gofer/ua-refresh.toml` |
| `/understand` 스킬 원문 (동작 근거) | `~/.claude/plugins/cache/understand-anything/understand-anything/2.9.4/skills/understand/SKILL.md` — Phase 0 결정표, 증분 경로, worktree 리다이렉트 |
| Claude Code headless 문서 | https://code.claude.com/docs/en/headless |
| launchd 예약 동작 | `man launchd.plist` → StartCalendarInterval |

## 8. 💡 다음 작업 힌트

환경 사실 (이 Mac):
- `claude` = `~/.local/bin/claude` (2.1.259). 사용자 alias `cl` = `claude --dangerously-skip-permissions`
- `node`/`pnpm` 은 nvm (`~/.nvm/versions/node/*/bin`), `go` 1.26.7, Homebrew `/opt/homebrew/bin`
- macOS 에 `timeout`/`gtimeout` 없음. `launchctl bootstrap gui/$(id -u) <plist>` / `bootout` 사용
- 이미 `~/Library/LaunchAgents` 에 다른 LaunchAgent 들이 있음 (충돌 없음)

코드 조각:

```go
// main.go
if err := fang.Execute(context.Background(), cmd.Root()); err != nil { os.Exit(1) }

// Charm v2 import 경로
import (
    tea "github.com/charmbracelet/bubbletea/v2"
    "github.com/charmbracelet/bubbles/v2/spinner"
    "github.com/charmbracelet/lipgloss/v2"
)

// 프로세스 그룹 단위 타임아웃
cmd := exec.CommandContext(ctx, "claude", "-p", prompt, "--dangerously-skip-permissions",
    "--output-format", "json", "--max-budget-usd", budget)
cmd.Dir = repoPath
cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }
cmd.WaitDelay = 10 * time.Second   // SIGTERM 후 유예, 지나면 SIGKILL
```

그래프 해시 읽기: `<repo>/.understand-anything/meta.json` 이 있으면 그것, 없으면 `<repo>/.ua/meta.json` 의 `gitCommitHash`. 해시 존재 확인: `git cat-file -e <hash>^{commit}`.

가짜 claude (테스트): PATH 맨 앞 임시 디렉토리에 `claude` 셸 스크립트를 두고, 인자를 파일에 기록한 뒤 `meta.json` 의 `gitCommitHash` 를 `git rev-parse HEAD` 로 갱신하고 `{"is_error":false,"total_cost_usd":0.5}` 를 출력하게 한다.
