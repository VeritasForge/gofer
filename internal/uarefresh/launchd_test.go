package uarefresh

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// installFakeLaunchctl 은 인자를 한 줄씩 누적 기록하는 가짜 launchctl 을 PATH 앞에 둔다.
// FAKE_LAUNCHCTL_PRINT_EXIT 로 `print` 의 종료 코드를 정한다 (기본 0).
func installFakeLaunchctl(t *testing.T) (argsFile string) {
	t.Helper()
	dir := t.TempDir()
	argsFile = filepath.Join(dir, "args")
	script := "#!/bin/sh\necho \"$*\" >> \"" + argsFile + "\"\n" +
		"if [ \"$1\" = print ]; then exit \"${FAKE_LAUNCHCTL_PRINT_EXIT:-0}\"; fi\n"
	if err := os.WriteFile(filepath.Join(dir, "launchctl"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return argsFile
}

func TestPlistXML(t *testing.T) {
	got := PlistXML(PlistParams{
		Label: "gofer.ua-refresh", Program: "/Users/example/.local/bin/gofer", Args: []string{"ua-refresh", "run"},
		Hour: 7, Minute: 30, LogPath: "/Users/example/Library/Logs/gofer/ua-refresh/launchd.log",
	})
	want := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>gofer.ua-refresh</string>
	<key>ProgramArguments</key>
	<array>
		<string>/Users/example/.local/bin/gofer</string>
		<string>ua-refresh</string>
		<string>run</string>
	</array>
	<key>StartCalendarInterval</key>
	<dict>
		<key>Hour</key>
		<integer>7</integer>
		<key>Minute</key>
		<integer>30</integer>
	</dict>
	<key>StandardOutPath</key>
	<string>/Users/example/Library/Logs/gofer/ua-refresh/launchd.log</string>
	<key>StandardErrorPath</key>
	<string>/Users/example/Library/Logs/gofer/ua-refresh/launchd.log</string>
</dict>
</plist>
`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestPlistXMLEscapes(t *testing.T) {
	got := PlistXML(PlistParams{Label: "x", Program: "/tmp/a&b/gofer", Args: []string{"<run>"}, LogPath: "/l"})
	if !strings.Contains(got, "/tmp/a&amp;b/gofer") || !strings.Contains(got, "&lt;run&gt;") {
		t.Errorf("not escaped:\n%s", got)
	}
}

func TestParseAt(t *testing.T) {
	if h, m, err := ParseAt("07:30"); err != nil || h != 7 || m != 30 {
		t.Errorf("got %d:%d %v", h, m, err)
	}
	if _, _, err := ParseAt("7:30pm"); err == nil {
		t.Error("want error for bad format")
	}
}

func TestInstallWritesPlistAndBootstraps(t *testing.T) {
	argsFile := installFakeLaunchctl(t)
	base := t.TempDir()
	paths := Paths{LogDir: filepath.Join(base, "logs"), Plist: filepath.Join(base, "LaunchAgents", "gofer.ua-refresh.plist")}
	if err := Install(t.Context(), paths, "07:30", "/usr/local/bin/gofer"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(paths.Plist)
	if err != nil || !strings.Contains(string(b), "<string>/usr/local/bin/gofer</string>") || !strings.Contains(string(b), "<integer>30</integer>") {
		t.Errorf("plist: %v\n%s", err, b)
	}
	if st, _ := os.Stat(paths.LogDir); st == nil || !st.IsDir() {
		t.Error("log dir should be created so launchd can open the log")
	}
	calls, _ := os.ReadFile(argsFile)
	want := fmt.Sprintf("bootout gui/%d/gofer.ua-refresh\nbootstrap gui/%d %s\n", os.Getuid(), os.Getuid(), paths.Plist)
	if string(calls) != want {
		t.Errorf("launchctl calls:\n%s\nwant:\n%s", calls, want)
	}
}

func TestUninstallBootsOutAndRemovesPlist(t *testing.T) {
	argsFile := installFakeLaunchctl(t)
	paths := Paths{Plist: filepath.Join(t.TempDir(), "gofer.ua-refresh.plist")}
	os.WriteFile(paths.Plist, []byte("x"), 0o644)
	if err := Uninstall(t.Context(), paths); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(paths.Plist); err == nil {
		t.Error("plist should be removed")
	}
	calls, _ := os.ReadFile(argsFile)
	if want := fmt.Sprintf("bootout gui/%d/gofer.ua-refresh\n", os.Getuid()); string(calls) != want {
		t.Errorf("launchctl calls:\n%s\nwant:\n%s", calls, want)
	}
}

func TestInstalled(t *testing.T) {
	installFakeLaunchctl(t)
	if ok, err := Installed(t.Context()); err != nil || !ok {
		t.Errorf("print exit 0 → installed; got %v %v", ok, err)
	}
	t.Setenv("FAKE_LAUNCHCTL_PRINT_EXIT", "113")
	if ok, err := Installed(t.Context()); err != nil || ok {
		t.Errorf("print exit 113 → not installed; got %v %v", ok, err)
	}
}
