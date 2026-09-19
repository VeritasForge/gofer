package holiday

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

// DefaultURL 은 대한민국 공휴일 달력이다. 설정에서 주소를 바꾸면 다른 국가·기관의 달력도
// 쓸 수 있지만, DESCRIPTION 이 한국어 낱말 "공휴일"로 시작하는 항목만 골라내므로
// (ParseICS 참고) 그 낱말을 쓰는 한국어 로캘 피드에만 실제로 통한다.
const DefaultURL = "https://calendar.google.com/calendar/ical/ko.south_korea%23holiday%40group.v.calendar.google.com/public/basic.ics"

// Config 는 ~/.config/gofer/holiday.toml 이다. 사람만 쓰고 프로그램은 읽기만 한다 —
// 그래서 sync 를 몇 번 돌려도 손으로 적어 둔 내용과 주석이 날아가지 않는다.
type Config struct {
	URL    string   `toml:"url"`
	Extra  []string `toml:"extra"`
	Ignore []string `toml:"ignore"`
}

// ConfigTemplate 은 `gofer holiday sync --init-config` 가 쓰는 설정 템플릿이다.
const ConfigTemplate = `# 내려받을 달력 주소. 비우면 대한민국 공휴일 달력을 쓴다.
url = "` + DefaultURL + `"

# 달력에 없지만 쉬는 날로 칠 날짜. 임시공휴일이 갑자기 지정됐을 때 여기에 적는다.
extra = []

# 달력에 있지만 쉬는 날로 치지 않을 날짜.
# 이 달력은 노동절(5/1)과 제헌절(7/17)을 공휴일로 싣는데, 연도마다 기준이 다르고
# 관공서 기준과도 어긋난다. 그날 일한다면 여기에 적는다.
ignore = []
`

// LoadConfig 는 설정을 읽는다. 파일이 없으면 기본값을 돌려준다 (오류가 아니다).
// 날짜 형식 오류와 모르는 항목은 전부 모아 하나의 오류로 돌려준다.
func LoadConfig(path string) (Config, error) {
	cfg := Config{URL: DefaultURL}
	md, err := toml.DecodeFile(path, &cfg)
	if errors.Is(err, os.ErrNotExist) {
		return Config{URL: DefaultURL}, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("read %s: %w", path, err)
	}
	var errs []error
	for _, k := range md.Undecoded() {
		errs = append(errs, fmt.Errorf("unknown key %q", k.String()))
	}
	if cfg.URL == "" {
		cfg.URL = DefaultURL
	}
	errs = append(errs, checkDates("extra", cfg.Extra)...)
	errs = append(errs, checkDates("ignore", cfg.Ignore)...)
	if len(errs) > 0 {
		return cfg, fmt.Errorf("config %s:\n%w", path, errors.Join(errs...))
	}
	return cfg, nil
}

func checkDates(field string, dates []string) []error {
	var errs []error
	for _, d := range dates {
		if _, err := time.Parse(dateLayout, d); err != nil {
			errs = append(errs, fmt.Errorf("%s: %q is not YYYY-MM-DD", field, d))
		}
	}
	return errs
}

// Range 는 공휴일 목록이 담고 있는 기간이다.
type Range struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// File 은 calendar.json 이다. sync 만 쓴다.
type File struct {
	SyncedAt string  `json:"synced_at"`
	Source   string  `json:"source"`
	Covers   Range   `json:"covers"`
	Holidays []Entry `json:"holidays"`
}

// ReadFile 은 저장된 목록을 읽는다.
func ReadFile(path string) (File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return File{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(f.Holidays) == 0 {
		return File{}, fmt.Errorf("%s has no holidays", path)
	}
	return f, nil
}

// WriteFile 은 목록을 저장한다. 디렉토리가 없으면 만든다.
func WriteFile(path string, f File) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// Open 은 설정과 저장된 목록을 함께 읽어 판정용 달력을 만든다.
//
// 저장된 목록이 없거나 깨졌거나 now 가 수록 기간 밖이면, 주말만 아는 달력과 사람이 읽을
// 경고 문구를 돌려준다 — 판정할 수 없다고 실행을 막으면 그래프가 조용히 며칠씩 늙기 때문이다.
// 오류를 돌려주는 경우는 설정 파일 자체가 잘못 적혀 있을 때뿐이다. 그건 사람이 고쳐야 한다.
func Open(p Paths, now time.Time) (*Calendar, string, error) {
	cfg, err := LoadConfig(p.Config)
	if err != nil {
		return nil, "", err
	}
	f, rerr := ReadFile(p.Calendar())
	if rerr != nil {
		return New(nil, "", "", cfg.Extra, cfg.Ignore), syncHint(rerr), nil
	}
	cal := New(f.Holidays, f.Covers.From, f.Covers.To, cfg.Extra, cfg.Ignore)
	if !cal.Covers(now) {
		return cal, fmt.Sprintf("holiday list only covers %s~%s — run `gofer holiday sync` (weekends still apply)", f.Covers.From, f.Covers.To), nil
	}
	return cal, "", nil
}

func syncHint(err error) string {
	if errors.Is(err, os.ErrNotExist) {
		return "no holiday list yet — run `gofer holiday sync` (weekends still apply)"
	}
	return fmt.Sprintf("could not read the holiday list (%v) — run `gofer holiday sync` (weekends still apply)", err)
}
