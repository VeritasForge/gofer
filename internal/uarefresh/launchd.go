package uarefresh

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// PlistParams 는 LaunchAgent plist 에 들어가는 값이다 (설계 3절 파일 위치, 2절 launchd 근거).
type PlistParams struct {
	Label   string
	Program string
	Args    []string
	Hour    int
	Minute  int
	LogPath string
}

// PlistXML 은 StartCalendarInterval 로 매일 한 번 실행되는 plist 본문이다. 잠든 동안 놓친 예약은 wake 시 실행된다.
func PlistXML(p PlistParams) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>` + xmlEscape(p.Label) + `</string>
	<key>ProgramArguments</key>
	<array>
		<string>` + xmlEscape(p.Program) + `</string>
`)
	for _, a := range p.Args {
		b.WriteString("\t\t<string>" + xmlEscape(a) + "</string>\n")
	}
	fmt.Fprintf(&b, `	</array>
	<key>StartCalendarInterval</key>
	<dict>
		<key>Hour</key>
		<integer>%d</integer>
		<key>Minute</key>
		<integer>%d</integer>
	</dict>
	<key>StandardOutPath</key>
	<string>%s</string>
	<key>StandardErrorPath</key>
	<string>%s</string>
</dict>
</plist>
`, p.Hour, p.Minute, xmlEscape(p.LogPath), xmlEscape(p.LogPath))
	return b.String()
}

func xmlEscape(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

// ParseAt 은 설정의 schedule.at ("HH:MM") 을 시·분으로 푼다.
func ParseAt(at string) (int, int, error) {
	t, err := time.Parse("15:04", at)
	if err != nil {
		return 0, 0, fmt.Errorf("schedule.at must be HH:MM, got %q", at)
	}
	return t.Hour(), t.Minute(), nil
}

func domainTarget() string { return fmt.Sprintf("gui/%d", os.Getuid()) }

func launchctl(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, "launchctl", args...).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("launchctl %s: %w: %s", strings.Join(args, " "), err, bytes.TrimSpace(out))
	}
	return string(out), nil
}

// Install 은 plist 를 쓰고 launchd 에 올린다. 이미 올라가 있으면 내렸다가 다시 올려 시각 변경을 반영한다.
func Install(ctx context.Context, paths Paths, at, program string) error {
	hour, minute, err := ParseAt(at)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(paths.LogDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(paths.Plist), 0o755); err != nil {
		return err
	}
	xmlBody := PlistXML(PlistParams{
		Label: LaunchdLabel, Program: program, Args: []string{"ua-refresh", "run"},
		Hour: hour, Minute: minute, LogPath: paths.LaunchdLog(),
	})
	if err := os.WriteFile(paths.Plist, []byte(xmlBody), 0o644); err != nil {
		return err
	}
	_, _ = launchctl(ctx, "bootout", domainTarget()+"/"+LaunchdLabel) // 안 올라가 있으면 실패하는 게 정상
	_, err = launchctl(ctx, "bootstrap", domainTarget(), paths.Plist)
	return err
}

// Uninstall 은 launchd 에서 내리고 plist 를 지운다.
func Uninstall(ctx context.Context, paths Paths) error {
	if _, err := launchctl(ctx, "bootout", domainTarget()+"/"+LaunchdLabel); err != nil {
		if _, statErr := os.Stat(paths.Plist); statErr == nil {
			return err // plist 는 있는데 못 내렸다 — 진짜 오류
		}
	}
	if err := os.Remove(paths.Plist); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Installed 는 launchd 에 작업이 올라가 있는지 본다.
func Installed(ctx context.Context) (bool, error) {
	cmd := exec.CommandContext(ctx, "launchctl", "print", domainTarget()+"/"+LaunchdLabel)
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
