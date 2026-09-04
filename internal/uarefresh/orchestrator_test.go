package uarefresh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gofer/internal/tui"
)

// runRepos 는 Run 을 돌리고 결과와 Event 목록을 돌려준다.
func runRepos(t *testing.T, f fakeClaude, repos ...RepoConfig) (RunResult, []tui.Event) {
	t.Helper()
	events := make(chan tui.Event, 64)
	res := Run(t.Context(), RunParams{
		Repos:     repos,
		Claude:    ClaudeConfig{BudgetUSD: 5, TimeoutMin: 1},
		ExtraPath: []string{f.Dir},
		Events:    events,
	})
	var evs []tui.Event
	for ev := range events {
		evs = append(evs, ev)
	}
	return res, evs
}

func runOne(t *testing.T, f fakeClaude, work, trunk string) (RepoResult, []tui.Event) {
	t.Helper()
	res, evs := runRepos(t, f, RepoConfig{Path: work, Trunk: trunk})
	if len(res.Repos) != 1 {
		t.Fatalf("repos = %d", len(res.Repos))
	}
	return res.Repos[0], evs
}

// trace 는 Event 목록을 "start stage:fetch ... done:✓ alldone" 꼴로 요약한다.
func trace(evs []tui.Event) string {
	var parts []string
	for _, ev := range evs {
		switch ev.Kind {
		case tui.KindStart:
			parts = append(parts, "start")
		case tui.KindStage:
			parts = append(parts, "stage:"+ev.Label)
		case tui.KindDone:
			parts = append(parts, "done:"+ev.Outcome.Symbol())
		case tui.KindAllDone:
			parts = append(parts, "alldone")
		}
	}
	return strings.Join(parts, " ")
}

func TestRunUpToDateDoesNotCallClaude(t *testing.T) {
	work, _ := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "ok")
	head, _ := Head(t.Context(), work)
	writeMeta(t, work, ".ua", head)
	rr, evs := runOne(t, f, work, "main")
	if rr.Status != StatusUpToDate || rr.CostUSD != 0 {
		t.Fatalf("result = %+v", rr)
	}
	if f.called() {
		t.Fatal("claude must not run when graph hash == HEAD")
	}
	if got := trace(evs); got != "start stage:fetch stage:merge done:– alldone" {
		t.Errorf("trace = %q", got)
	}
}

func TestRunUpdatesWhenOriginAhead(t *testing.T) {
	work, origin := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "ok")
	want := pushFromClone(t, origin, "main", "a.txt", "a")
	rr, evs := runOne(t, f, work, "main")
	if rr.Status != StatusUpdated || rr.Commits != 1 || rr.CostUSD != 0.5 || rr.Elapsed <= 0 {
		t.Fatalf("result = %+v", rr)
	}
	if h, _ := Head(t.Context(), work); h != want {
		t.Errorf("HEAD = %s want %s (ff-merge did not happen)", h, want)
	}
	if args := f.argsLines(t); args[1] != "/understand" {
		t.Errorf("prompt = %q", args[1])
	}
	if got := trace(evs); got != "start stage:fetch stage:merge stage:/understand … done:✓ alldone" {
		t.Errorf("trace = %q", got)
	}
	done := evs[len(evs)-2]
	if done.Label != "graph updated" || done.Detail != "+1 commit" || done.CostUSD != 0.5 || done.Elapsed <= 0 {
		t.Errorf("done event = %+v", done)
	}
}

func TestRunUsesFullWhenGraphCommitMissing(t *testing.T) {
	work, _ := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "ok")
	writeMeta(t, work, ".ua", "0123456789abcdef0123456789abcdef01234567")
	rr, evs := runOne(t, f, work, "main")
	if rr.Status != StatusUpdated {
		t.Fatalf("result = %+v", rr)
	}
	if args := f.argsLines(t); args[1] != "/understand --full" {
		t.Errorf("prompt = %q", args[1])
	}
	if !strings.Contains(trace(evs), "stage:/understand --full …") {
		t.Errorf("trace = %q", trace(evs))
	}
}

func TestRunSkipsWhenBranchMismatch(t *testing.T) {
	work, _ := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "ok")
	run(t, work, "git", "checkout", "-q", "-b", "feature-x")
	rr, evs := runOne(t, f, work, "main")
	if rr.Status != StatusSkipped || !strings.Contains(rr.Reason, `"feature-x"`) {
		t.Fatalf("result = %+v", rr)
	}
	if f.called() || strings.Contains(trace(evs), "fetch") {
		t.Fatalf("guard must stop before fetch/claude; trace = %q", trace(evs))
	}
}

func TestRunSkipsWhenTrackedChanges(t *testing.T) {
	work, _ := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "ok")
	os.WriteFile(filepath.Join(work, "README.md"), []byte("edited"), 0o644)
	rr, _ := runOne(t, f, work, "main")
	if rr.Status != StatusSkipped || rr.Reason != "uncommitted changes in tracked files" {
		t.Fatalf("result = %+v", rr)
	}
}

func TestRunFailsWhenDiverged(t *testing.T) {
	work, origin := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "ok")
	commitFile(t, work, "local.txt", "l")
	pushFromClone(t, origin, "main", "remote.txt", "r")
	rr, _ := runOne(t, f, work, "main")
	if rr.Status != StatusFailed || rr.Reason != "ff-merge failed: 1 local commit ahead of origin" {
		t.Fatalf("result = %+v", rr)
	}
	if f.called() {
		t.Fatal("claude must not run after merge failure")
	}
}

func TestRunFailsWhenGraphHashNotUpdated(t *testing.T) {
	work, origin := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "noop")
	pushFromClone(t, origin, "main", "a.txt", "a")
	rr, _ := runOne(t, f, work, "main")
	if rr.Status != StatusFailed || !strings.Contains(rr.Reason, "graph hash still not at HEAD") || rr.CostUSD != 0.1 {
		t.Fatalf("result = %+v", rr)
	}
}

func TestRunFailsWhenClaudeReportsError(t *testing.T) {
	work, origin := newRepoWithOrigin(t, "main")
	f := installFakeClaude(t, "error")
	pushFromClone(t, origin, "main", "a.txt", "a")
	rr, _ := runOne(t, f, work, "main")
	if rr.Status != StatusFailed || !strings.Contains(rr.Reason, "budget exceeded") || rr.CostUSD != 0.2 {
		t.Fatalf("result = %+v", rr)
	}
}

func TestRunContinuesAfterFailure(t *testing.T) {
	bad, badOrigin := newRepoWithOrigin(t, "main")
	commitFile(t, bad, "local.txt", "l")
	pushFromClone(t, badOrigin, "main", "remote.txt", "r")
	good, _ := newRepoWithOrigin(t, "develop")
	head, _ := Head(t.Context(), good)
	writeMeta(t, good, ".ua", head)
	f := installFakeClaude(t, "ok")
	res, evs := runRepos(t, f, RepoConfig{Path: bad, Trunk: "main"}, RepoConfig{Path: good, Trunk: "develop"})
	if res.Repos[0].Status != StatusFailed || res.Repos[1].Status != StatusUpToDate {
		t.Fatalf("results = %+v", res.Repos)
	}
	if res.ExitCode() != 1 {
		t.Error("exit code should be 1")
	}
	if got := trace(evs); got != "start stage:fetch stage:merge done:✗ start stage:fetch stage:merge done:– alldone" {
		t.Errorf("trace = %q", got)
	}
}

func TestGuardReasonPasses(t *testing.T) {
	work, _ := newRepoWithOrigin(t, "main")
	reason, err := GuardReason(t.Context(), RepoConfig{Path: work, Trunk: "main"})
	if err != nil || reason != "" {
		t.Fatalf("got (%q, %v)", reason, err)
	}
}
