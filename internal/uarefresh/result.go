package uarefresh

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"gofer/internal/tui"
)

// Status 는 레포 하나의 결과다 (설계 4절).
type Status string

const (
	StatusUpdated  Status = "updated"    // 그래프 갱신
	StatusUpToDate Status = "up-to-date" // 변경 없음, Claude 호출 안 함
	StatusSkipped  Status = "skipped"    // 가드에 걸림
	StatusFailed   Status = "failed"     // merge·Claude·검증 실패
)

// RepoResult 는 레포 하나의 처리 결과다. last-run.json 에 그대로 저장된다.
type RepoResult struct {
	Name    string        `json:"name"`
	Trunk   string        `json:"trunk"`
	Status  Status        `json:"status"`
	Commits int           `json:"commits"`
	Reason  string        `json:"reason,omitempty"`
	Elapsed time.Duration `json:"elapsed_ns"`
	CostUSD float64       `json:"cost_usd"`
}

// RunResult 는 한 번의 실행 전체다.
type RunResult struct {
	StartedAt  time.Time    `json:"started_at"`
	FinishedAt time.Time    `json:"finished_at"`
	Repos      []RepoResult `json:"repos"`
	LogPath    string       `json:"log_path"`
}

// Labels 는 화면·DM 요약에 쓰는 결과별 문구다.
var Labels = tui.Labels{OK: "updated", Noop: "up to date", Skipped: "skipped", Failed: "failed"}

// Counts 는 결과별 개수와 총비용이다.
type Counts struct {
	Updated, UpToDate, Skipped, Failed int
	CostUSD                            float64
}

func (r RunResult) Counts() Counts {
	var c Counts
	for _, rr := range r.Repos {
		switch rr.Status {
		case StatusUpdated:
			c.Updated++
		case StatusUpToDate:
			c.UpToDate++
		case StatusSkipped:
			c.Skipped++
		case StatusFailed:
			c.Failed++
		}
		c.CostUSD += rr.CostUSD
	}
	return c
}

func (c Counts) String() string {
	return tui.Summary(Labels, c.Updated, c.UpToDate, c.Skipped, c.Failed, c.CostUSD)
}

// ExitCode 는 skipped 나 failed 가 하나라도 있으면 1 이다 (설계 4절).
func (r RunResult) ExitCode() int {
	c := r.Counts()
	if c.Skipped > 0 || c.Failed > 0 {
		return 1
	}
	return 0
}

// WriteRunResult 는 last-run.json 을 쓴다.
func WriteRunResult(path string, r RunResult) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// ReadRunResult 는 last-run.json 을 읽는다.
func ReadRunResult(path string) (RunResult, error) {
	var r RunResult
	b, err := os.ReadFile(path)
	if err != nil {
		return r, err
	}
	return r, json.Unmarshal(b, &r)
}
