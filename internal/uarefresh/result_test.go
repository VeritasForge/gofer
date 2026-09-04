package uarefresh

import (
	"path/filepath"
	"testing"
	"time"
)

func sampleRun() RunResult {
	return RunResult{
		StartedAt:  time.Date(2026, 9, 5, 8, 30, 0, 0, time.Local),
		FinishedAt: time.Date(2026, 9, 5, 8, 41, 0, 0, time.Local),
		Repos: []RepoResult{
			{Name: "example-api", Trunk: "main", Status: StatusUpdated, Commits: 13, Elapsed: 252 * time.Second, CostUSD: 1.83},
			{Name: "example-web", Trunk: "develop", Status: StatusUpdated, Commits: 2, Elapsed: 125 * time.Second, CostUSD: 0.71},
			{Name: "example-docs", Trunk: "main", Status: StatusUpToDate, Elapsed: 2 * time.Second},
			{Name: "example-infra", Trunk: "main", Status: StatusFailed, Reason: "ff-merge failed: 1 local commit ahead of origin", Elapsed: 3 * time.Second},
		},
		LogPath: "/home/example/Library/Logs/gofer/ua-refresh/2026-09-05.log",
	}
}

func TestCountsAndExitCode(t *testing.T) {
	r := sampleRun()
	if got := r.Counts().String(); got != "2 updated · 1 up to date · 0 skipped · 1 failed · $2.54" {
		t.Errorf("counts = %q", got)
	}
	if r.ExitCode() != 1 {
		t.Error("failed repo should give exit 1")
	}
	r.Repos = r.Repos[:3]
	if r.ExitCode() != 0 {
		t.Error("all good should give exit 0")
	}
	r.Repos[0].Status = StatusSkipped
	if r.ExitCode() != 1 {
		t.Error("skipped repo should give exit 1")
	}
}

func TestRunResultRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "last-run.json")
	if err := WriteRunResult(path, sampleRun()); err != nil {
		t.Fatal(err)
	}
	got, err := ReadRunResult(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Repos) != 4 || got.Repos[0].CostUSD != 1.83 || !got.FinishedAt.Equal(sampleRun().FinishedAt) {
		t.Errorf("round trip mismatch: %+v", got)
	}
}
