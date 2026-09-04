// Package uarefresh 는 ua-refresh 도구의 로직이다. cmd/ 는 이 패키지를 호출만 한다.
package uarefresh

import (
	"os"
	"path/filepath"
	"time"
)

// Paths 는 설계 3절 "파일 위치" 표다. 테스트는 임시 디렉토리로 직접 만든다.
type Paths struct {
	Config   string // ~/.config/gofer/ua-refresh.toml
	LogDir   string // ~/Library/Logs/gofer/ua-refresh
	StateDir string // ~/Library/Application Support/gofer/ua-refresh
	Plist    string // ~/Library/LaunchAgents/gofer.ua-refresh.plist
}

// LaunchdLabel 은 launchd 작업 이름이다.
const LaunchdLabel = "gofer.ua-refresh"

// DefaultPaths 는 홈 디렉토리 기준 기본 위치를 만든다.
func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	return Paths{
		Config:   filepath.Join(home, ".config", "gofer", "ua-refresh.toml"),
		LogDir:   filepath.Join(home, "Library", "Logs", "gofer", "ua-refresh"),
		StateDir: filepath.Join(home, "Library", "Application Support", "gofer", "ua-refresh"),
		Plist:    filepath.Join(home, "Library", "LaunchAgents", LaunchdLabel+".plist"),
	}, nil
}

func (p Paths) DailyLog(t time.Time) string { return filepath.Join(p.LogDir, t.Format("2006-01-02")+".log") }
func (p Paths) LaunchdLog() string          { return filepath.Join(p.LogDir, "launchd.log") }
func (p Paths) Lock() string                { return filepath.Join(p.StateDir, "run.lock") }
func (p Paths) LastRun() string             { return filepath.Join(p.StateDir, "last-run.json") }
