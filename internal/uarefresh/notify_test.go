package uarefresh

import (
	"os"
	"path/filepath"
	"testing"
)

// installFakeOsascript 는 PATH 맨 앞에 인자를 기록하는 가짜 osascript 를 둔다. (Task 12 도 재사용)
func installFakeOsascript(t *testing.T) (argsFile string) {
	t.Helper()
	dir := t.TempDir()
	argsFile = filepath.Join(dir, "args")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"" + argsFile + "\"\n"
	if err := os.WriteFile(filepath.Join(dir, "osascript"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argsFile
}

func TestMacNotifyBuildsAppleScript(t *testing.T) {
	argsFile := installFakeOsascript(t)
	if err := MacNotify(t.Context(), "ua-refresh", `2 updated · 1 failed "quoted"`); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(argsFile)
	want := "-e\ndisplay notification \"2 updated · 1 failed \\\"quoted\\\"\" with title \"ua-refresh\"\n"
	if string(b) != want {
		t.Errorf("args:\n%s\nwant:\n%s", b, want)
	}
}
