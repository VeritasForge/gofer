# gofer ua-refresh 설계

작성일: 2026-09-04
상태: 설계 승인됨, 구현 플랜 작성 전

## 1. 무엇을, 왜

`gofer`는 개발자가 매일 반복하는 잔심부름을 대신하는 개인용 CLI 모음이다. 이름은 "go for this, go for that"의 *gofer*(심부름꾼)와 Go 마스코트 gopher의 발음을 겹친 것이다. 바이너리 하나에 도구가 서브커맨드로 붙는 2단 구조다(`gofer <도구> <동작>`).

첫 도구 `ua-refresh`는 여러 git 레포의 **Understand-Anything 지식 그래프**(`.ua/knowledge-graph.json`, Claude Code 플러그인 `understand-anything`이 만드는 코드베이스 구조 색인)를 매일 아침 자동으로 최신 trunk 기준으로 갱신한다. 지금은 사람이 매일 아침 레포마다 다음을 손으로 한다.

1. `git fetch -ptf`
2. trunk 브랜치로 이동
3. `git merge --ff-only origin/<trunk>`
4. `claude -p "/understand"` 실행 (레포당 수 분에서 십수 분)

이 네 단계를 launchd 예약 작업으로 자동화하고, 결과를 Slack DM으로 받는다.

### 목표

- 매일 정해진 시각(기본 07:30)에 설정된 모든 레포를 순서대로 처리한다.
- 변경이 없는 레포는 Claude를 호출하지 않는다(비용 0).
- 사용자의 작업 상태를 절대 망치지 않는다. 조건이 안 맞는 레포는 건너뛰고 알린다.
- 끝나면 Slack DM 한 통으로 결과를 요약한다.
- 터미널에서 직접 실행하면 진행 상황이 보기 좋게 표시된다.

### 목표가 아닌 것

- 자동 기상(`pmset`), 레포 병렬 처리, 주말 제외, Slack 봇 토큰 방식, 실패 시 자동 재시도, 다른 Understand-Anything 스킬(`/understand-domain` 등) 갱신. 필요해지면 설정 한 줄이나 작은 함수로 붙일 수 있는 구조로 둔다.

## 2. 전제 조건

| 전제 | 근거·확인 방법 |
|---|---|
| 각 레포의 **루트 작업 트리는 항상 trunk 브랜치**에 있고, 기능 작업은 전부 git worktree에서 한다 | 사용자 규칙. 이 전제 덕에 checkout 없이 ff-merge만으로 안전하다. 어긋나면 그 레포는 건너뛴다 |
| trunk 브랜치 이름은 레포마다 다르다 (`main`, `develop`, `staging` 등) | origin/HEAD가 실제 통합 브랜치와 다른 레포가 있으므로 **자동 추론하지 않고 설정에 명시**한다 |
| `.ua/`(또는 레거시 `.understand-anything/`)는 git이 추적하지 않는다 | ff-merge와 충돌하지 않는다. 추적 중이라면 이 도구의 범위 밖 |
| macOS, Claude Code CLI(`claude`) 로그인 상태, `understand-anything` 플러그인 설치, Node.js·pnpm 존재 | `/understand` 스킬의 요구사항 |
| Slack Incoming Webhook 하나(대상: 본인 DM) | 사용자가 직접 발급 |

### `/understand` 스킬에서 확인한 동작 (플러그인 2.9.4 기준)

- 데이터 디렉토리는 `.understand-anything/`이 있으면 그것, 없으면 `.ua/`.
- `meta.json`의 `gitCommitHash`가 마지막 분석 커밋이다.
- 그 해시가 HEAD와 **같으면 사용자에게 "전체 재빌드 / 리뷰 / 아무것도 안 함"을 묻고 멈춘다.** 무인 실행에서는 답할 사람이 없으므로 **이 도구가 먼저 비교해서 같으면 Claude를 호출하지 않는다.**
- 다르면 `git diff <해시>..HEAD --name-only`로 바뀐 파일만 증분 분석한다. 그 해시가 레포에 없으면(force-push 등) 증분이 불가능하므로 `--full`을 넘긴다.
- 그래프 파일은 마지막 단계에서만 쓰므로, 도중에 죽어도 기존 그래프는 남는다.

### Claude Code 무인 실행에서 확인한 사실 (CLI 2.1.259, 공식 headless 문서 기준)

- `claude -p "/understand"`처럼 프롬프트에 스킬명을 넣으면 플러그인 스킬이 확장된다. `--bare`는 플러그인을 건너뛰므로 쓰지 않는다.
- `/understand`는 Bash·서브에이전트·파일 쓰기를 모두 쓰므로 `--dangerously-skip-permissions`가 필요하다. `--permission-mode dontAsk`는 허용 규칙 밖 동작을 거부해 맞지 않는다.
- `--output-format json`의 결과에 `is_error`, `total_cost_usd`가 있다. `--max-budget-usd`로 지출 상한을 건다.
- launchd에서 Keychain 로그인이 읽히는지는 문서로 단정할 수 없다. **구현 첫 단계에서 실험**하고, 안 되면 `claude setup-token`으로 만든 토큰을 `CLAUDE_CODE_OAUTH_TOKEN` 환경변수로 넘긴다.
- launchd는 잠든 동안 놓친 예약을 깨어날 때 실행한다(`man launchd.plist`, StartCalendarInterval). cron은 건너뛰므로 launchd를 쓴다.
- launchd의 PATH는 비어 있다시피 하므로 `claude`, `node`, `pnpm` 경로를 도구가 직접 구성한다.

## 3. 사용법

```
gofer ua-refresh run [--dry-run] [--only <레포 디렉토리명>]   본 작업. 터미널이면 TUI, 아니면 한 줄 로그
gofer ua-refresh config init                                 설정 템플릿 생성 (~/.config/gofer/ua-refresh.toml, 0600)
gofer ua-refresh install | uninstall                         launchd 작업 등록/해제
gofer ua-refresh status                                      등록 여부, 마지막 실행 결과, 레포별 그래프 신선도
gofer ua-refresh log [--follow]                              오늘 로그
```

`--help`, `--version`, 에러 출력, 셸 자동완성(`gofer completion`)은 fang이 제공한다.

### 설정 파일 `~/.config/gofer/ua-refresh.toml`

```toml
[schedule]
at = "07:30"                 # install 이 plist 에 반영

[claude]
budget_usd  = 20             # 레포당 지출 상한 (폭주 방지용)
timeout_min = 60             # 레포당 시간 상한 (fetch·merge·/understand 전체)
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
```

- 필수: `repos` 1개 이상, 각 `path`(존재하는 git 레포)와 `trunk`, `notify.slack.webhook_url`. 누락·오류는 `run` 시작 전에 전부 모아서 보고한다.
- 레포 이름(`--only`, 화면, DM)은 `path`의 마지막 디렉토리명이다.

### 파일 위치

| 무엇 | 경로 |
|---|---|
| 설정 | `~/.config/gofer/ua-refresh.toml` |
| 일별 로그, launchd stdout/stderr | `~/Library/Logs/gofer/ua-refresh/YYYY-MM-DD.log`, `launchd.log` |
| 상태(잠금 파일, 마지막 실행 결과) | `~/Library/Application Support/gofer/ua-refresh/run.lock`, `last-run.json` |
| launchd plist | `~/Library/LaunchAgents/gofer.ua-refresh.plist` (label `gofer.ua-refresh`) |

## 4. 동작

### 전체 흐름

```
launchd (설정 시각, 잠들었으면 wake 직후) ──▶ gofer ua-refresh run
                                               │ run.lock 획득 (이미 실행 중이면 즉시 종료)
                                               ▼
   ┌─ 설정의 각 레포, 순서대로 ────────────────────────────────────────────┐
   │ ① 가드: 현재 브랜치 == trunk ?  추적 파일에 미커밋 수정 없음 ?        │
   │      아니오 ──▶ 건너뜀(guard) + 사유                                  │
   │ ② git fetch -ptf                                                    │
   │ ③ git merge --ff-only origin/<trunk>   실패(분기·로컬 커밋) ──▶ 실패   │
   │ ④ 그래프 해시 == HEAD ?                                              │
   │      같음 ──▶ 최신(up-to-date), Claude 호출 안 함                     │
   │      해시가 레포에 없음 ──▶ --full 로 실행                             │
   │ ⑤ claude -p "/understand[ --full]" --dangerously-skip-permissions     │
   │        --output-format json --max-budget-usd N [--model M]           │
   │      cwd = 레포 루트, 시간 상한 timeout_min, 프로세스 그룹 단위 종료   │
   │ ⑥ 검증: 그래프 해시 == HEAD 면 갱신(updated), 아니면 실패(failed)      │
   └────────────────────────────────────────────────────────────────────┘
                                               │
                                               ▼
                       last-run.json 기록 · Slack DM 1건 · 잠금 해제 · 종료 코드
```

- 레포 결과는 네 가지: `updated`(그래프 갱신), `up-to-date`(변경 없음), `skipped`(가드에 걸림), `failed`(merge·Claude·검증 실패).
- 한 레포의 실패가 다른 레포를 막지 않는다. `skipped`나 `failed`가 하나라도 있으면 종료 코드 1, 아니면 0.
- 요청의 "trunk로 checkout" 단계는 가드가 대신한다. 전제상 루트는 항상 trunk이고, 아니라면 이상 상황이므로 checkout하지 않고 건너뛰어 알린다.
- 미커밋 수정 판정은 추적 파일만 본다(`git status --porcelain --untracked-files=no`). 미추적 파일은 허용한다.
- `--dry-run`은 git과 Claude를 전혀 건드리지 않는다. 설정 검증과 가드 결과, 레포별로 "무엇을 할지"만 출력한다.

### Claude 호출 세부

- 환경: 설정의 `extra_path`를 PATH 앞에 붙인다. `oauth_token`이 있으면 `CLAUDE_CODE_OAUTH_TOKEN`으로 넘긴다.
- stdout(JSON)은 파싱해 `is_error`, `total_cost_usd`를 기록하고, stderr는 로그로 보낸다.
- 시간 상한 초과 시 프로세스 그룹에 SIGTERM, 잠시 후 SIGKILL. `/understand`가 여러 서브셸을 띄우므로 그룹 단위가 아니면 고아 프로세스가 남는다.
- 성공 판정은 Claude의 종료 코드가 아니라 **그래프 해시가 HEAD와 같아졌는지**로 한다. 스킬이 중간에 실패해도 정상 종료할 수 있기 때문이다.

## 5. 코드 구조

```
gofer/                                모듈 하나(module gofer), 바이너리 하나
├── go.mod                            Go 1.26
├── main.go                           fang.Execute(cmd.Root())
├── cmd/
│   ├── root.go                       `gofer` 루트 커맨드. 도구 목록이 --help 에 나온다
│   └── uarefresh/                    `gofer ua-refresh` 서브커맨드 트리. 플래그 파싱만, 로직 없음
├── internal/
│   ├── uarefresh/                    도구 로직: 설정, git, Claude 실행, 오케스트레이터, launchd, DM 문구
│   ├── tui/                          "항목 N개 순차 처리 + 상태 표시" 화면. Event 소비. TTY면 bubbletea, 아니면 한 줄 로그
│   └── slack/                        Incoming Webhook POST
├── *_test.go                         각 패키지 옆
├── docs/                             이 설계, 구현 플랜, 인계 문서
├── justfile                          build / test / lint (개발용. 설치·실행은 바이너리가 직접)
└── bin/                              빌드 결과 (git 제외)
```

- `cmd/`는 얇게, `internal/`에 로직. cobra 없이 로직을 테스트하기 위해서다.
- 오케스트레이터는 화면을 모른다. `Event`(레포 시작 · 단계 전환 · 레포 완료 · 전체 완료)만 채널로 발행하고, TUI와 plain 로그가 같은 Event를 소비한다. 그래서 두 화면이 어긋날 수 없고, 테스트는 Event 스트림으로 한다.
- `internal/tui`, `internal/slack`은 도구 공용이다. 다음 도구가 같은 패턴이면 그대로 쓴다.
- 새 도구 추가 = `cmd/<도구>/` + `internal/<도구>/` 만들고 `cmd/root.go`에 한 줄 등록.

### 의존성 (직접 6개, Go 모듈 프록시에서 확인한 최신 버전)

| 역할 | 모듈 (import 경로) | 버전 |
|---|---|---|
| CLI 프레임워크 | `github.com/spf13/cobra` | v1.10.2 |
| help·에러·version·완성·man 스타일링 | `github.com/charmbracelet/fang` | v1.0.0 (README에 "experimental" 표기, 의존성은 모두 안정) |
| 실행 화면 엔진 | `charm.land/bubbletea/v2` | v2.0.9 |
| 스피너 | `charm.land/bubbles/v2` | v2.2.1 |
| 색·표 | `charm.land/lipgloss/v2` | v2.0.6 |
| 설정 파싱 | `github.com/BurntSushi/toml` | v1.6.0 |

Charm v2 계열 셋은 GitHub 저장소는 `charmbracelet/` 아래에 있지만 go.mod 가 모듈 경로를 `charm.land/...` 로 선언한다(2026-09-04 플랜 작성 시 확인 — `github.com/charmbracelet/lipgloss/v2` 로 `go get` 하면 "module declares its path as: charm.land/lipgloss/v2" 로 거부된다). fang 은 `github.com/charmbracelet/fang` 그대로다.

## 6. 화면, 알림, 로그

### 터미널 화면 (`gofer ua-refresh run`)

```
 ua-refresh · 6 repos · 08:41:02

 ✓ example-api     main       +13 commits   graph updated    4m12s   $1.83
 ✓ example-web     develop    +2 commits    graph updated    2m05s   $0.71
 – example-docs    main       up to date
 ⠹ example-infra   main       +5 commits    /understand …    1m37s
 · example-mobile  release    waiting
 · example-lib     main       waiting

 ─────────────────────────────────────────────────────────────────
 2 updated · 1 up to date · 0 skipped · 0 failed · $2.54 so far
```

레포마다 한 줄, 처리 중인 줄에만 스피너와 경과 시간. 끝나면 요약 표. TTY가 아니면(launchd) 같은 내용을 한 줄씩 로그로 쓴다.

### Slack DM (실행 종료 시 1건, 항상)

```
ua-refresh 2026-09-05 08:41 · 2 updated · 1 up to date · 0 skipped · 1 failed · $2.54
✓ example-api    main      +13 commits  4m12s  $1.83
✓ example-web    develop   +2 commits   2m05s  $0.71
– example-docs   main      up to date
✗ example-infra  main      ff-merge failed: local commits ahead of origin
로그: ~/Library/Logs/gofer/ua-refresh/2026-09-05.log
```

DM 전송이 실패하면 macOS 알림(`osascript`) 한 줄로 대체한다. 로그는 항상 남는다.

### 로그

일별 파일에 레포별 git 출력 요약, Claude 결과 JSON 요약, 예외를 기록한다. launchd 자체의 stdout/stderr는 `launchd.log`.

## 7. 안전장치

| 상황 | 처리 |
|---|---|
| 이미 실행 중 (wake 직후 중복 등) | `run.lock`에 pid 기록. lock이 있는데 그 pid가 죽어 있으면 stale로 보고 제거 |
| 설정 오류 | 시작 전에 전부 모아 보고하고 아무것도 하지 않음 |
| 브랜치 불일치 · 미커밋 수정 | 그 레포만 `skipped`, 사유 기록 |
| ff-merge 실패 | 그 레포만 `failed`, Claude 호출 안 함 |
| Claude 시간·예산 초과, 비정상 종료 | 프로세스 그룹 종료, 그 레포 `failed`. 기존 그래프는 그대로 남고 다음 날 재시도 |
| Slack 전송 실패 | macOS 알림 + 로그 |

## 8. 테스트와 검증

| 단계 | 방법 | 통과 기준 |
|---|---|---|
| 단위 | `go test ./...`, `go vet`. `t.TempDir()`에 임시 git 레포(bare origin 포함). **가짜 `claude`** 는 PATH 맨 앞에 둔 셸 스크립트로, 실제 exec 경로·인자까지 검증 | 가드 2종(브랜치·미커밋), ff 실패, 변경 없음, `--full` 분기, 타임아웃 시 그룹 종료, 설정 검증, DM 문구, plist 내용 |
| 화면 | TUI 모델의 `View()` 문자열 비교(golden) | 상태 전이별 렌더링 |
| 인증 실험 (구현 첫 단계) | launchd로 `claude -p "reply ok" --output-format json` 1회 | JSON 수신. 실패하면 `oauth_token` 경로로 전환 |
| 통합 | `config init` → `run --dry-run` → `run --only <레포>` → `run` → `install` | DM 수신, 그래프 해시 == HEAD |
| 운영 | 다음 날 아침 | wake 후 DM 도착, 로그 정상 |

## 9. 결정 기록

| 결정 | 이유 |
|---|---|
| Go | 사용자가 이 레포를 Go CLI 모음으로 키우려 함. 단일 바이너리, `exec.CommandContext`로 타임아웃이 깔끔 |
| cobra + fang (kong 대신) | 2단 서브커맨드 트리와 자동완성이 cobra의 강점. fang이 help·에러 화면을 공짜로 스타일링. kong은 typer와 더 닮았지만 화면을 직접 꾸며야 함 |
| Charm v2 계열 | v2가 정식 릴리스되어 v1을 새로 쓸 이유가 없음 |
| 바이너리 하나 + 서브커맨드 | `gh`, `kubectl` 방식. 도구가 늘어도 설치·설정 디렉토리가 하나 |
| 설정은 TOML 한 파일 | 주석 가능, 사람이 편집. 비밀값(웹훅)도 같은 파일(0600) |
| launchd (cron 대신) | 잠든 동안 놓친 예약을 wake 시 실행 |
| Claude 호출 전에 해시 비교 | `/understand`가 변경 없을 때 사용자에게 되묻고 멈추므로, 무인 실행에서는 도구가 먼저 걸러야 함. 변경 없는 날 비용 0 |
| 별도 worktree에서 그래프 갱신 안 함 | `/understand`의 worktree 리다이렉트는 출력 위치가 아니라 **분석 대상 루트 자체**를 본 레포로 바꾸므로 worktree 방식이 성립하지 않음. 루트=trunk 전제로 해결 |
| trunk 자동 추론 안 함 | origin/HEAD가 실제 통합 브랜치와 다른 레포가 있음 |
| 회사 중립 | 코드·주석·문서·예제에 특정 조직 정보 없음. 실제 레포 목록은 설정 파일에만 |
