package uarefresh

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestStatusText(t *testing.T) {
	fresh, _ := newRepoWithOrigin(t, "main")
	freshHead, _ := Head(t.Context(), fresh)
	writeMeta(t, fresh, ".ua", freshHead)
	stale, _ := newRepoWithOrigin(t, "develop")
	writeMeta(t, stale, ".ua", "0123456789abcdef0123456789abcdef01234567")
	none, _ := newRepoWithOrigin(t, "main")

	base := t.TempDir()
	paths := Paths{StateDir: filepath.Join(base, "state"), Plist: filepath.Join(base, "gofer.ua-refresh.plist")}
	if err := WriteRunResult(paths.LastRun(), sampleRun()); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Schedule: ScheduleConfig{At: "07:30"}, Repos: []RepoConfig{{Path: fresh, Trunk: "main"}, {Path: stale, Trunk: "develop"}, {Path: none, Trunk: "main"}}}

	got := StatusText(t.Context(), paths, cfg, true)
	for _, want := range []string{
		"launchd: installed · daily at 07:30",
		"last run: 2026-09-05 08:30 → 08:41 · 2 updated · 1 up to date · 0 skipped · 1 failed · $2.54 · exit 1",
		"HEAD " + freshHead[:7] + "  graph " + freshHead[:7] + "  fresh",
		"graph 0123456  stale",
		"graph -        none",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}

	got = StatusText(t.Context(), Paths{StateDir: t.TempDir()}, cfg, false)
	if !strings.Contains(got, "launchd: not installed") || !strings.Contains(got, "last run: never") {
		t.Errorf("got:\n%s", got)
	}
}
