package uarefresh

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gofer/internal/holiday"
)

// StatusText 는 `gofer ua-refresh status` 본문이다: launchd 등록 여부 · 오늘 판정 ·
// 마지막 실행 · 레포별 그래프 신선도.
func StatusText(ctx context.Context, paths Paths, hpaths holiday.Paths, cfg *Config, installed bool, now time.Time) string {
	var b strings.Builder
	only := ""
	if cfg.Schedule.WorkdaysOnly {
		only = " · workdays only"
	}
	if installed {
		fmt.Fprintf(&b, "launchd: installed · daily at %s%s · %s\n", cfg.Schedule.At, only, shortenHome(paths.Plist))
	} else {
		b.WriteString("launchd: not installed (run `gofer ua-refresh install`)\n")
	}
	if cfg.Schedule.WorkdaysOnly {
		b.WriteString(todayLine(hpaths, now))
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

// todayLine 은 오늘이 쉬는 날인지 한 줄로 알려 준다. 목록이 없거나 낡았으면 그 사실도 붙인다.
func todayLine(hpaths holiday.Paths, now time.Time) string {
	cal, warning, err := holiday.Open(hpaths, now)
	if err != nil {
		return fmt.Sprintf("today:   %s · holiday config error: %v\n", now.Format("2006-01-02 (Mon)"), err)
	}
	line := fmt.Sprintf("today:   %s · workday\n", now.Format("2006-01-02 (Mon)"))
	if reason, off := cal.Holiday(now); off {
		line = fmt.Sprintf("today:   %s · day off (%s) · not running today\n", now.Format("2006-01-02 (Mon)"), reason)
	}
	if warning != "" {
		line += "         " + warning + " (실제 run은 이 목록을 갱신한 뒤 다시 판정한다)\n"
	}
	return line
}
