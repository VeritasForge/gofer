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
		d := rr.Elapsed.Round(time.Second)
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		elapsed := fmt.Sprintf("%dm%02ds", mins, secs)
		return strings.TrimRight(fmt.Sprintf("%-12s  %s  $%.2f", commitsText(rr.Commits), elapsed, rr.CostUSD), " ")
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
