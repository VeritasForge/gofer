package holiday

import (
	"os"
	"path/filepath"
)

// Paths 는 holiday 도구가 쓰는 파일 위치다. 테스트는 임시 디렉토리로 직접 만든다.
type Paths struct {
	Config   string // ~/.config/gofer/holiday.toml — 사람만 쓴다
	StateDir string // ~/Library/Application Support/gofer/holiday — sync 만 쓴다
}

// Calendar 는 내려받은 목록을 저장하는 파일이다.
func (p Paths) Calendar() string { return filepath.Join(p.StateDir, "calendar.json") }

// DefaultPaths 는 홈 디렉토리 기준 기본 위치를 만든다.
func DefaultPaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, err
	}
	return Paths{
		Config:   filepath.Join(home, ".config", "gofer", "holiday.toml"),
		StateDir: filepath.Join(home, "Library", "Application Support", "gofer", "holiday"),
	}, nil
}
