package uarefresh

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gofer/internal/holiday"
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

	got := StatusText(t.Context(), paths, holiday.Paths{}, cfg, true, time.Now())
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

	got = StatusText(t.Context(), Paths{StateDir: t.TempDir()}, holiday.Paths{}, cfg, false, time.Now())
	if !strings.Contains(got, "launchd: not installed") || !strings.Contains(got, "last run: never") {
		t.Errorf("got:\n%s", got)
	}
}

// TestStatusTextShowsTodayOff 는 쉬는 날에 status 가 그 사실을 알려 주는지 본다.
func TestStatusTextShowsTodayOff(t *testing.T) {
	cfg := &Config{Schedule: ScheduleConfig{At: "07:30", WorkdaysOnly: true}}
	base := t.TempDir()
	hpaths := holiday.Paths{
		Config:   filepath.Join(base, "holiday.toml"),
		StateDir: filepath.Join(base, "holiday"),
	}
	saturday := time.Date(2026, 9, 12, 7, 30, 0, 0, time.Local)
	got := StatusText(t.Context(), Paths{StateDir: base}, hpaths, cfg, true, saturday)

	if !strings.Contains(got, "workdays only") {
		t.Errorf("status should say the schedule is workdays only:\n%s", got)
	}
	if !strings.Contains(got, "주말") || !strings.Contains(got, "not running today") {
		t.Errorf("status should show today's judgement:\n%s", got)
	}
}

// TestStatusTextOmitsTodayWhenOff 는 기능을 껐을 때 오늘 줄이 나오지 않는지 본다.
func TestStatusTextOmitsTodayWhenOff(t *testing.T) {
	cfg := &Config{Schedule: ScheduleConfig{At: "07:30", WorkdaysOnly: false}}
	base := t.TempDir()
	hpaths := holiday.Paths{Config: filepath.Join(base, "holiday.toml"), StateDir: filepath.Join(base, "holiday")}
	saturday := time.Date(2026, 9, 12, 7, 30, 0, 0, time.Local)
	got := StatusText(t.Context(), Paths{StateDir: base}, hpaths, cfg, true, saturday)

	if strings.Contains(got, "workdays only") || strings.Contains(got, "not running today") {
		t.Errorf("with workdays_only off, status should not mention it:\n%s", got)
	}
}
