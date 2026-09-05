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
