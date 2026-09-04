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
	// tui.Run 은 어느 경로로 끝나든 events 를 다 비운 뒤에만 돌아오므로, 그 뒤에 Log 를 닫아도 안전하다.
	uiErr := tui.Run(ctx, tui.Options{
		Title: fmt.Sprintf("ua-refresh · %d repos", len(repos)), Items: items, Labels: Labels,
		Out: o.Stdout, Log: logw, TTY: o.TTY, Now: now, Cancel: cancel,
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
