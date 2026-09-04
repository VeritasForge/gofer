# gofer

개인용 Go CLI 모음. 바이너리 하나(`gofer`)에 도구가 서브커맨드로 붙는다: `gofer <도구> <동작>`.

## 절대 규칙: 조직 중립

이 레포는 공개 가능한 개인 도구다. **코드·주석·README·테스트·예제·문서 어디에도 특정 회사·팀·레포·브랜치·워크스페이스 정보를 쓰지 않는다.** 예제는 `~/src/example-api`, `main`처럼 중립적으로 쓴다. 환경에 따라 달라지는 값(경로, 시각, 예산, 토큰)은 전부 사용자 설정 파일(`~/.config/gofer/<도구>.toml`)로 뺀다.

## 기술 결정 (바꾸려면 설계 문서부터 고친다)

- Go 1.26, 모듈 하나(`module gofer`), 외부 의존성은 cobra · fang · bubbletea/v2 · bubbles/v2 · lipgloss/v2 · BurntSushi/toml 여섯 개.
- `cmd/`는 플래그 파싱만, 로직은 `internal/<도구>/`. 오케스트레이터는 화면을 모르고 Event만 발행한다.
- `internal/tui`, `internal/slack`은 도구 공용.
- 새 도구 = `cmd/<도구>/` + `internal/<도구>/` + `cmd/root.go`에 한 줄.

## 검증

- `go test ./...` 와 `go vet ./...` 가 통과해야 한다. 외부 프로세스(`git`, `claude`)는 임시 레포와 PATH 앞의 가짜 실행 파일로 테스트한다.

## 문서

- 설계: `docs/superpowers/specs/`
- 구현 플랜: `docs/superpowers/plans/`
- 세션 인계: `docs/autopilot/<도구>/HANDOFF.md`
