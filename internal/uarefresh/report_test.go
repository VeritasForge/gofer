package uarefresh

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStartText(t *testing.T) {
	started := time.Date(2026, 9, 5, 7, 30, 4, 0, time.Local)
	got := StartText(started, 6)
	want := "ua-refresh 2026-09-05 07:30 · starting · 6 repos"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestStartTextSingleRepo(t *testing.T) {
	got := StartText(time.Date(2026, 9, 5, 7, 30, 0, 0, time.Local), 1)
	want := "ua-refresh 2026-09-05 07:30 · starting · 1 repo"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestSlackText(t *testing.T) {
	home, _ := os.UserHomeDir()
	r := sampleRun()
	r.LogPath = filepath.Join(home, "Library/Logs/gofer/ua-refresh/2026-09-05.log")
	want := `ua-refresh 2026-09-05 08:41 · 2 updated · 1 up to date · 0 skipped · 1 failed · $2.54
✓ example-api    main     +13 commits   4m12s  $1.83
✓ example-web    develop  +2 commits    2m05s  $0.71
– example-docs   main     up to date
✗ example-infra  main     ff-merge failed: 1 local commit ahead of origin
로그: ~/Library/Logs/gofer/ua-refresh/2026-09-05.log`
	if got := SlackText(r); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestSlackTextSkippedRow(t *testing.T) {
	r := RunResult{Repos: []RepoResult{{Name: "example-api", Trunk: "main", Status: StatusSkipped, Reason: "uncommitted changes in tracked files"}}}
	got := SlackText(r)
	if want := "↷ example-api  main  skipped: uncommitted changes in tracked files"; !strings.Contains(got, want) {
		t.Errorf("got:\n%s\nwant line:\n%s", got, want)
	}
}
