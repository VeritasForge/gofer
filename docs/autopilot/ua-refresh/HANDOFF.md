# HANDOFF — gofer ua-refresh

## 1. ⚡ 즉시 재진입

구현과 통합 검증이 모두 끝났고 launchd 예약(매일 07:30)도 등록돼 있다. 다음 세션이 할 일은:

1. `gofer ua-refresh status` 로 `last run` 이 오늘 날짜인지 확인 — 밤사이 launchd 가 정상 동작했는지 보는 것.
2. `/demiurge:rl` 로 플랜 파일(`docs/superpowers/plans/2026-09-04-ua-refresh-implementation.md`) 맨 위 "플랜 전체 완료조건" 표(A~E)를 검증.
3. 통과하면 `superpowers:finishing-a-development-branch` 스킬로 `main` 병합 여부를 정한다(브랜치 `worktree-ua-refresh`).

코드나 설정을 더 바꿀 필요는 없다. 아래는 그 판단에 필요한 배경이다.

## 2. 📍 TL;DR — 어디서 멈췄나

- 브랜치: `worktree-ua-refresh` (아직 `main` 에 병합 전)
- 구현(플랜 Task 2~14) 완료 → 최종 리뷰 1회(치명적 결함 1건 발견해 즉시 수정, 재검증 통과) → 실제 레포로 통합 검증까지 전부 끝남
- launchd 등록됨: `gofer ua-refresh install` 실행 완료, 매일 07:30 예약
- 남은 건 "다음 날 아침 DM 이 실제로 왔는지"와 "브랜치를 언제 합칠지" 뿐

## 3. ✅ 완료

| 작업 | 요약 |
|---|---|
| 브레인스토밍 · 설계 승인 | 대화로 진행, 결과는 설계 문서 9절 "결정 기록"에 정리 |
| 설계 문서 · 프로젝트 규칙 | `docs/superpowers/specs/2026-09-04-ua-refresh-design.md`, `CLAUDE.md` |
| 구현 플랜 작성 | `docs/superpowers/plans/2026-09-04-ua-refresh-implementation.md` — 15 Task, 완료조건·검증 명령 포함 |
| 인증 실험 (Task 1) | launchd 에서 Keychain 인증으로 됨. 토큰 발급 불필요 |
| 구현 (Task 2~14) | 서브에이전트 기반 개발로 진행. Task 마다 구현→리뷰→완료조건 검증을 거쳤고, 리뷰에서 나온 지연 항목은 원장(아래 참고)에 판정과 함께 남겨둠 |
| 최종 전체 리뷰 | 치명적 결함 1건(전송 실패 시 비밀값이 로그로 샐 수 있던 것) + 중요 1건(레포당 시간 상한이 일부 단계만 감쌌던 것) 발견 → 한 번의 수정으로 해결, 범위 한정 재검증 통과 |
| 통합 검증 (Task 15) | 아래 4절 참고 |

## 4. ⏳ 남은 작업

| 작업 | 핵심 액션 |
|---|---|
| 다음 날 아침 확인 | `gofer ua-refresh status` 로 launchd 가 실제로 돈 흔적(오늘 날짜의 `last run`) 확인, DM 도착 확인 |
| 플랜 전체 완료조건 최종 검증 | `/demiurge:rl` 로 플랜 파일 상단 A~E 표 검증 |
| 병합 결정 | `superpowers:finishing-a-development-branch` 로 `main` 병합 여부 결정 |

통합 검증(Task 15) 중 실제로 있었던 일:
- 실제 레포 6개로 `--dry-run` → 레포 1개(`--only`) → 전체 `run` → `install` 순서로 진행.
- 첫 실행에서 `/understand` 스킬이 사용자 확인을 기다리며 질문만 남기고 끝나는 문제를 발견 — 무인 실행 지시(`--append-system-prompt`)를 추가해 해결(설계 2·4절에 반영, 아래 5절에도 기록).
- 레포 하나가 예산 상한에 걸려 실패 — Claude 구독 요금제(Team/Enterprise 좌석)에서는 `--max-budget-usd`/`total_cost_usd` 가 실제 청구액이 아니라 로컬 추정치이고, 진짜 제한은 좌석당 5시간/주간 사용량 한도라는 걸 확인(아래 8절). 모델을 더 저렴한 쪽으로, 예산 상한을 넉넉히 조정해 재실행 후 성공.
- 최종적으로 설정된 레포 전부 그래프가 최신 상태(HEAD 와 그래프 해시 일치)가 됨을 확인.
- launchd 등록·직접 의존성 개수·조직 중립 검색까지 전부 실측 통과.

## 5. 🛡 누적 judgments

| 결정 | 적용 위치 |
|---|---|
| 조직 중립: 코드·주석·문서·예제에 회사/레포/브랜치/워크스페이스 정보 금지. 실제 값은 설정 파일에만 | `CLAUDE.md`, 설계 1·9절 |
| 실제 레포 목록·trunk·webhook 은 `~/.config/gofer/ua-refresh.toml` 에만 있음(0600, git 밖) | 설정 파일 |
| 설정 스키마가 구현 중 바뀌면 그 파일도 같이 고칠 것. `config init` 은 기존 파일을 덮어쓰지 않는다 | 설계 3절 |
| 루트 작업 트리 = 항상 trunk (기능 작업은 worktree). 어긋나면 그 레포 skip | 설계 2·4절 |
| trunk 는 설정에 명시, origin/HEAD 로 추론 금지 | 설계 2절 |
| Claude 호출 전에 그래프 해시 == HEAD 비교, 같으면 호출 안 함 | 설계 2·4절 |
| 그래프 커밋이 레포에 없으면 `--full` | 설계 4절 |
| worktree 에서 `/understand` 돌리는 방식은 성립 안 함 | 설계 9절 |
| `--bare` 금지, `--dangerously-skip-permissions` 필요, `--max-budget-usd` 사용 | 설계 2절 |
| **무인 실행 지시 추가**: `/understand` 가 확인 대기 상태에서 질문만 남기고 끝나는 걸 막기 위해 `--append-system-prompt` 로 "확인 요청은 기본값으로 진행, 대시보드 실행 금지" 를 명시 | 설계 2·4절, 통합 검증에서 발견 |
| **실패 시 원인 노출**: Claude 응답의 첫 줄을 실패 사유와 로그에 남긴다 — 질문으로 끝났는지 예산 초과인지 DM 만 보고 알 수 있게 | 설계 4절, 최종 리뷰 반영 |
| 성공 판정은 Claude 종료 코드가 아니라 그래프 해시 == HEAD | 설계 4절 |
| **시간 상한은 레포 단계 전체**(fetch·merge·claude)를 감싼다 — claude 단계만 감쌌다가 fetch 가 멈추면 잠금이 안 풀리는 문제를 최종 리뷰에서 발견해 수정 | 설계 3·4절, 최종 리뷰 반영 |
| 타임아웃은 프로세스 그룹 단위 종료 | 설계 4절 |
| launchd (cron 아님), 잠들면 wake 시 실행 | 설계 1·2절 |
| Slack DM 은 매 실행 종료 시 항상 1건, 실패 시 osascript. **웹훅 URL 은 절대 로그에 남기지 않는다** — 전송 실패 시 오류 문구에 URL 이 섞여 나오는 걸 최종 리뷰에서 발견해 차단 | 설계 6절, 최종 리뷰 반영 |

## 6. 🚨 외부 review 패턴

브랜치 전체를 한 번 종합 리뷰했고, 결함 1건(치명)·1건(중요)·10건(경미)이 나왔다. 치명·중요 건은 그 자리에서 고치고 회귀 테스트를 추가한 뒤 범위를 좁혀 다시 검증받아 통과했다. 경미한 건 중 코드 변경으로 이어진 것은 같은 수정에 묶었고, 나머지는 실질 위험이 낮다고 판단해 보류(플랜 파일 원장에 판정 이유와 함께 기록)했다. 남은 보류 항목 중 눈여겨볼 것: 프로세스가 예기치 않게 재부팅된 뒤에는 잠금 파일이 사람 손으로 지워야 할 수 있다(설계 규칙 자체의 빈틈).

## 7. 🗂 핵심 파일 위치

| 무엇 | 경로 |
|---|---|
| 설계 문서 | `docs/superpowers/specs/2026-09-04-ua-refresh-design.md` |
| 구현 플랜 (진행 체크박스 포함) | `docs/superpowers/plans/2026-09-04-ua-refresh-implementation.md` |
| 프로젝트 규칙 | `CLAUDE.md` |
| 실제 사용자 설정 (git 밖) | `~/.config/gofer/ua-refresh.toml` |
| `/understand` 스킬 원문 (동작 근거) | `~/.claude/plugins/cache/understand-anything/understand-anything/2.9.4/skills/understand/SKILL.md` |
| Claude Code headless 문서 | https://code.claude.com/docs/en/headless |
| launchd 예약 동작 | `man launchd.plist` → StartCalendarInterval |

## 8. 💡 다음 작업 힌트

환경 사실 (이 Mac):
- `claude` = `~/.local/bin/claude`. `node`/`pnpm` 은 nvm, `go` 1.26.7, Homebrew `/opt/homebrew/bin`
- macOS 에 `timeout`/`gtimeout` 없음. `launchctl bootstrap gui/$(id -u) <plist>` / `bootout` 사용
- launchd 인증: Keychain 으로 됨, `oauth_token` 설정 불필요
- **구독 요금제(Team/Enterprise 좌석)에서 `total_cost_usd`/`--max-budget-usd` 는 실제 청구액이 아니라 토큰 사용량을 정가로 환산한 로컬 추정치다.** 진짜 제한은 좌석당 5시간 롤링 + 주간 사용량 할당량이고, 다 쓰면 `session limit` 오류가 뜬다 — `budget_usd` 와 완전히 별개 메커니즘. 그래도 `budget_usd` 는 레포 하나가 공유 할당량을 통째로 먹는 걸 막는 로컬 안전판으로 의미가 있다(공식 문서 `code.claude.com/docs/en/costs.md`, `errors.md` 참고).
- 통합 검증에서 레포별로 변경량 차이가 커서, 예산·모델 설정을 상황에 맞게 조정했다. 큰 변경분이 있는 레포에서 기본 예산이 부족하면 `budget_usd` 를 올리거나 더 저렴한 모델로 바꾸는 걸 고려.

코드 조각:

```go
// main.go
if err := fang.Execute(context.Background(), cmd.Root()); err != nil { os.Exit(1) }

// Charm v2 import 경로 — go.mod 가 charm.land/... 로 모듈 경로를 선언한다 (github.com/charmbracelet/... 로 go get 하면 거부됨)
import (
    tea "charm.land/bubbletea/v2"
    "charm.land/bubbles/v2/spinner"
    "charm.land/lipgloss/v2"
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
