package holiday

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newPaths 는 임시 디렉토리에 설정과 저장 파일 위치를 잡는다. body 가 비면 설정 파일을 만들지 않는다.
func newPaths(t *testing.T, body string) Paths {
	t.Helper()
	dir := t.TempDir()
	p := Paths{Config: filepath.Join(dir, "holiday.toml"), StateDir: filepath.Join(dir, "state")}
	if body != "" {
		if err := os.WriteFile(p.Config, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

// storeSample 은 테스트에서 저장 파일로 쓰는 목록이다.
var storeSample = File{
	SyncedAt: "2026-09-19T10:00:00+09:00",
	Source:   DefaultURL,
	Covers:   Range{From: "2026-01-01", To: "2026-12-31"},
	Holidays: []Entry{
		{Date: "2026-03-01", Name: "삼일절"},
		{Date: "2026-03-02", Name: "쉬는 날 삼일절"},
		{Date: "2026-05-01", Name: "노동절"},
	},
}

func TestLoadConfigMissingFileUsesDefaults(t *testing.T) {
	cfg, err := LoadConfig(newPaths(t, "").Config)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.URL != DefaultURL || len(cfg.Extra) != 0 || len(cfg.Ignore) != 0 {
		t.Errorf("missing config should fall back to defaults, got %+v", cfg)
	}
}

func TestLoadConfigRejectsBadDates(t *testing.T) {
	_, err := LoadConfig(newPaths(t, "extra = [\"2026-13-99\"]\nignore = [\"nope\"]\n").Config)
	if err == nil {
		t.Fatal("bad dates should be an error")
	}
	for _, want := range []string{"2026-13-99", "nope"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q, got: %v", want, err)
		}
	}
}

func TestLoadConfigRejectsUnknownKey(t *testing.T) {
	if _, err := LoadConfig(newPaths(t, "urls = \"x\"\n").Config); err == nil {
		t.Fatal("unknown key should be an error")
	}
}

func TestWriteThenReadFile(t *testing.T) {
	p := newPaths(t, "")
	if err := WriteFile(p.Calendar(), storeSample); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := ReadFile(p.Calendar())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(got.Holidays) != 3 || got.Holidays[0].Name != "삼일절" || got.Covers.To != "2026-12-31" {
		t.Errorf("round trip lost data: %+v", got)
	}
}

func TestOpenAppliesConfig(t *testing.T) {
	p := newPaths(t, "extra = [\"2026-10-12\"]\nignore = [\"2026-05-01\"]\n")
	if err := WriteFile(p.Calendar(), storeSample); err != nil {
		t.Fatal(err)
	}
	cal, warning, err := Open(p, day(t, "2026-03-03"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if warning != "" {
		t.Errorf("a fresh list should not warn, got %q", warning)
	}
	if reason, off := cal.Holiday(day(t, "2026-03-02")); !off || reason != "쉬는 날 삼일절" {
		t.Errorf("stored holiday: got (%q, %v)", reason, off)
	}
	if !cal.IsWorkday(day(t, "2026-05-01")) {
		t.Error("ignore should remove 2026-05-01")
	}
	if cal.IsWorkday(day(t, "2026-10-12")) {
		t.Error("extra should add 2026-10-12")
	}
}

func TestOpenWithoutStoredList(t *testing.T) {
	p := newPaths(t, "")
	cal, warning, err := Open(p, day(t, "2026-03-02"))
	if err != nil {
		t.Fatalf("Open should not fail without a stored list: %v", err)
	}
	if !strings.Contains(warning, "gofer holiday sync") {
		t.Errorf("warning should point at sync, got %q", warning)
	}
	if !cal.IsWorkday(day(t, "2026-03-02")) {
		t.Error("a public holiday cannot be detected without a stored list")
	}
	if cal.IsWorkday(day(t, "2026-03-14")) {
		t.Error("saturday is still a day off")
	}
}

func TestOpenOutsideCoveredRange(t *testing.T) {
	p := newPaths(t, "")
	if err := WriteFile(p.Calendar(), storeSample); err != nil {
		t.Fatal(err)
	}
	_, warning, err := Open(p, day(t, "2027-03-02"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if !strings.Contains(warning, "gofer holiday sync") {
		t.Errorf("a stale list should warn, got %q", warning)
	}
}

func TestOpenFailsOnBrokenConfig(t *testing.T) {
	p := newPaths(t, "extra = [\"nope\"]\n")
	if _, _, err := Open(p, day(t, "2026-03-03")); err == nil {
		t.Fatal("a broken config must be an error — a person has to fix it")
	}
}

func TestOpenSurvivesBrokenStoredList(t *testing.T) {
	p := newPaths(t, "")
	if err := os.MkdirAll(p.StateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.Calendar(), []byte("{ broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	cal, warning, err := Open(p, day(t, "2026-03-02"))
	if err != nil {
		t.Fatalf("a broken stored list is a warning, not an error: %v", err)
	}
	if !strings.Contains(warning, "gofer holiday sync") {
		t.Errorf("warning should point at sync, got %q", warning)
	}
	if cal.IsWorkday(day(t, "2026-03-14")) {
		t.Error("saturday is still a day off")
	}
}
