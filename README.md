# gofer

[![License](https://img.shields.io/github/license/VeritasForge/gofer)](LICENSE)
[![Go version](https://img.shields.io/github/go-mod/go-version/VeritasForge/gofer)](go.mod)

개인용 Go CLI 잔심부름꾼 — 매일 아침 여러 git 레포를 최신 trunk로 맞추고 코드 지식 그래프까지 자동 갱신한다.

바이너리 하나(`gofer`)에 도구가 서브커맨드로 붙는 구조다: `gofer <도구> <동작>`. 첫 도구는 `ua-refresh`로, 여러 git 레포의 Understand-Anything 지식 그래프(`.ua/knowledge-graph.json` — 코드베이스 구조를 그래프로 색인하는 Claude Code 플러그인이 만드는 파일)를 매일 아침 자동으로 최신 상태로 갱신한다.

## Table of Contents

- [Background](#background)
- [Security](#security)
- [Install](#install)
- [Usage](#usage)
- [Configuration](#configuration)
- [Options](#options)
- [Exit Status](#exit-status)
- [Contributing](#contributing)
- [License](#license)

## Background

레포마다 매일 아침 다음 네 단계를 손으로 반복하던 것을 자동화하기 위해 만들었다.

1. `git fetch -ptf`
2. trunk 브랜치(레포의 통합 브랜치, 예: `main`)로 이동
3. `git merge --ff-only origin/<trunk>` (fast-forward-only 병합 — 로컬에 커밋이 없을 때만 되감기 없이 앞으로만 이동하는 안전한 병합)
4. `claude -p "/understand"` 실행 (지식 그래프 갱신, 레포당 수 분~십수 분)

이 네 단계를 macOS의 예약 실행 데몬인 `launchd` 작업으로 자동화하고, 결과를 Slack DM으로 받는다. 변경이 없는 레포는 Claude를 호출하지 않으므로 그만큼 비용도 들지 않는다.

이름 `gofer`는 잔심부름을 뜻하는 영어 단어 *gofer*("go for this, go for that")와 Go 언어 마스코트 gopher의 발음을 겹친 말장난이다.

설계 배경과 전제 조건은 `docs/superpowers/specs/2026-09-04-ua-refresh-design.md`에 더 자세히 있다.

## Security

`gofer ua-refresh config init`이 만드는 설정 파일 `~/.config/gofer/ua-refresh.toml`에는 Slack Webhook URL과, 필요할 때만 채우는 Claude Code OAuth 토큰이 평문으로 들어갈 수 있다. 이 파일은 `0600` 권한(소유자만 읽기·쓰기)으로 생성되지만, git에 커밋하거나 다른 사람과 공유해서는 안 된다.

## Install

Go 1.26.7 이상이 필요하다(`go.mod` 기준).

```bash
git clone https://github.com/VeritasForge/gofer.git
cd gofer
go build -o bin/gofer .
```

[`just`](https://github.com/casey/just)가 설치되어 있으면 `just build`로도 같은 결과를 얻는다 — 차이는 버전 문자열을 `git describe`로 자동 채워준다는 점뿐이다.

## Usage

```
gofer ua-refresh config init                                  설정 템플릿 생성 (~/.config/gofer/ua-refresh.toml, 0600)
gofer ua-refresh run [--dry-run] [--only <레포 디렉토리명>]    본 작업 실행 (터미널이면 TUI—터미널 안에 그려지는 대화형 화면, 아니면 한 줄 로그)
gofer ua-refresh install | uninstall                           launchd 작업 등록/해제 (macOS 전용)
gofer ua-refresh status                                        등록 여부·마지막 실행 결과·레포별 그래프 신선도 표시
gofer ua-refresh log [--follow]                                 오늘 로그 출력
```

### 첫 실행 예시

```bash
gofer ua-refresh config init             # ~/.config/gofer/ua-refresh.toml 생성
$EDITOR ~/.config/gofer/ua-refresh.toml  # repos 목록 · webhook_url 채우기
gofer ua-refresh run --dry-run           # git·claude 를 건드리지 않고 설정만 검증
gofer ua-refresh run                     # 실제 실행
gofer ua-refresh install                 # 매일 아침 자동 실행되도록 launchd 에 등록
```

`--help`, `--version`, 셸 자동완성(`gofer completion`)은 모든 커맨드에 공통이며 [fang](https://github.com/charmbracelet/fang)(cobra 기반 CLI에 도움말·버전 출력을 입혀주는 라이브러리)이 제공한다.

## Configuration

`gofer ua-refresh config init`이 만드는 템플릿이다. `repos` 항목은 예시일 뿐이니 실제 레포 경로로 바꿔야 한다.

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

| 항목 | 의미 |
|---|---|
| `schedule.at` | 매일 실행 시각(`HH:MM`). `gofer ua-refresh install`이 이 값으로 launchd 작업을 등록한다 |
| `claude.budget_usd` | 레포 1개당 Claude 지출 상한(달러). 넘으면 그 레포 처리를 중단한다 |
| `claude.timeout_min` | 레포 1개당 시간 상한(분). fetch·merge·`/understand` 전체에 적용한다 |
| `claude.oauth_token` | launchd에서 Keychain 로그인이 안 될 때만 채운다. 비워두면 `claude` CLI의 기본 로그인 상태를 그대로 쓴다 |
| `notify.slack.webhook_url` | 실행 결과를 받을 Slack Incoming Webhook (본인 DM 등) |
| `env.extra_path` | launchd는 `PATH`가 거의 비어 있어 `claude`·`node`·`pnpm` 등을 직접 찾도록 앞에 붙여주는 경로 목록. `*` 글롭은 이름순 정렬 후 마지막 항목만 쓴다 |
| `repos[].path` | 대상 레포의 루트 작업 트리 경로. 이 경로는 항상 trunk 브랜치에 있어야 한다(기능 작업은 별도 git worktree에서 한다) |
| `repos[].trunk` | 그 레포의 통합 브랜치 이름(`main`, `develop` 등 — 레포마다 다를 수 있어 자동 추론하지 않고 명시한다) |

## Options

| 커맨드 | 플래그 | 설명 |
|---|---|---|
| `run` | `--dry-run` | git과 claude를 건드리지 않고 설정 검증과 예상 동작만 출력한다 |
| `run` | `--only <레포 디렉토리명>` | 설정된 레포 중 이름이 일치하는 하나만 처리한다 |
| `log` | `-f`, `--follow` | 오늘 로그를 계속 따라가며 출력한다(Ctrl+C로 중단) |

## Exit Status

`gofer ua-refresh run`은 레포를 하나라도 건너뛰거나 실패하면 **1**을, 전부 정상 처리(또는 변경이 없어 스킵)되면 **0**을 반환한다. 그 외 커맨드는 성공 시 0, 잘못된 인자나 설정 오류 시 0이 아닌 값을 반환한다.

## Contributing

개인용 도구지만 이슈·PR은 환영한다. 이 저장소는 조직 중립을 지킨다 — 코드·문서·예제 어디에도 특정 회사·팀·레포·워크스페이스 정보를 넣지 않는다(자세한 규칙은 `CLAUDE.md` 참고). PR을 보내기 전에 아래를 통과시킨다.

```bash
go test ./...
go vet ./...
```

## License

[MIT](LICENSE) © 2026 VeritasForge
