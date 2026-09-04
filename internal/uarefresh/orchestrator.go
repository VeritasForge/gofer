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
