package uarefresh

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// Config 는 ~/.config/gofer/ua-refresh.toml 전체다 (설계 3절).
type Config struct {
	Schedule ScheduleConfig `toml:"schedule"`
	Claude   ClaudeConfig   `toml:"claude"`
	Notify   NotifyConfig   `toml:"notify"`
	Env      EnvConfig      `toml:"env"`
	Repos    []RepoConfig   `toml:"repos"`
}

type ScheduleConfig struct {
	At string `toml:"at"` // "HH:MM"
}

type ClaudeConfig struct {
	BudgetUSD  float64 `toml:"budget_usd"`
	TimeoutMin int     `toml:"timeout_min"`
	Model      string  `toml:"model"`
	OAuthToken string  `toml:"oauth_token"`
}

// Timeout 은 레포당 시간 상한이다.
func (c ClaudeConfig) Timeout() time.Duration { return time.Duration(c.TimeoutMin) * time.Minute }

type NotifyConfig struct {
	Slack SlackConfig `toml:"slack"`
}

type SlackConfig struct {
	WebhookURL string `toml:"webhook_url"`
}

type EnvConfig struct {
	ExtraPath []string `toml:"extra_path"`
}

type RepoConfig struct {
	Path  string `toml:"path"` // Load 후에는 ~ 가 풀린 경로 (그 외 형태는 그대로)
	Trunk string `toml:"trunk"`
}

// Name 은 --only, 화면, DM 에 쓰는 레포 이름(마지막 디렉토리명)이다.
func (r RepoConfig) Name() string { return filepath.Base(r.Path) }

// Template 은 `config init` 이 쓰는 설정 템플릿이다 (설계 3절 원문).
const Template = `[schedule]
at = "07:30"                 # install 이 plist 에 반영

[claude]
budget_usd  = 20             # 레포당 지출 상한 (폭주 방지용)
timeout_min = 60             # 레포당 시간 상한 (fetch·merge·/understand 전체)
model       = ""             # 비우면 Claude Code 기본 설정
oauth_token = ""             # launchd 에서 Keychain 인증이 안 될 때만

[notify.slack]
webhook_url = "https://hooks.slack.com/services/..."

[env]
# launchd 의 빈 PATH 를 보완. glob 은 이름순 정렬 후 마지막(최신) 항목을 고른다.
extra_path = ["~/.local/bin", "~/.nvm/versions/node/*/bin", "/opt/homebrew/bin"]

[[repos]]
path  = "~/src/example-api"
trunk = "main"

[[repos]]
path  = "~/src/example-web"
trunk = "develop"
`

// Load 는 설정을 읽고 ~ 를 풀고 검증한다. 오류는 전부 모아 하나로 돌려준다 (설계 3·7절).
func Load(path string) (*Config, error) {
	var cfg Config
	md, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var errs []error
	for _, k := range md.Undecoded() {
		errs = append(errs, fmt.Errorf("unknown key %q", k.String()))
	}
	for i := range cfg.Repos {
		cfg.Repos[i].Path = ExpandHome(cfg.Repos[i].Path)
	}
	for i := range cfg.Env.ExtraPath {
		cfg.Env.ExtraPath[i] = ExpandHome(cfg.Env.ExtraPath[i])
	}
	errs = append(errs, Validate(&cfg)...)
	if len(errs) > 0 {
		return nil, fmt.Errorf("config %s:\n%w", path, errors.Join(errs...))
	}
	return &cfg, nil
}

// Validate 는 필수 항목과 형식을 검사해 문제를 전부 돌려준다.
func Validate(c *Config) []error {
	var errs []error
	if _, err := time.Parse("15:04", c.Schedule.At); err != nil {
		errs = append(errs, fmt.Errorf("schedule.at must be HH:MM, got %q", c.Schedule.At))
	}
	if c.Claude.BudgetUSD <= 0 {
		errs = append(errs, errors.New("claude.budget_usd must be > 0"))
	}
	if c.Claude.TimeoutMin <= 0 {
		errs = append(errs, errors.New("claude.timeout_min must be > 0"))
	}
	if u := c.Notify.Slack.WebhookURL; !strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "http://") {
		errs = append(errs, errors.New("notify.slack.webhook_url is required (http(s) URL)"))
	} else if strings.Contains(u, "...") {
		errs = append(errs, errors.New("notify.slack.webhook_url still has the template placeholder"))
	}
	if len(c.Repos) == 0 {
		errs = append(errs, errors.New("at least one [[repos]] entry is required"))
	}
	for i, r := range c.Repos {
		if st, err := os.Stat(filepath.Join(r.Path, ".git")); err != nil || !st.IsDir() {
			errs = append(errs, fmt.Errorf("repos[%d].path %q is not a git repository root", i, r.Path))
		}
		if r.Trunk == "" {
			errs = append(errs, fmt.Errorf("repos[%d].trunk is required", i))
		}
	}
	return errs
}

// WriteTemplate 은 템플릿을 0600 으로 새로 만든다. 이미 있으면 os.ErrExist 를 돌려주고 건드리지 않는다.
func WriteTemplate(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(Template)
	return err
}

// ExpandHome 은 앞의 "~" 를 홈 디렉토리로 바꾼다.
func ExpandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

// ResolveExtraPath 는 각 항목을 glob 으로 풀어 이름순 마지막 하나를 고른다. 매치가 없으면 뺀다.
func ResolveExtraPath(entries []string) []string {
	var out []string
	for _, e := range entries {
		matches, _ := filepath.Glob(e)
		if len(matches) == 0 {
			continue
		}
		sort.Strings(matches)
		out = append(out, matches[len(matches)-1])
	}
	return out
}
