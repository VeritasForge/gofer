package uarefresh

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrNoGraph 는 그래프 디렉토리나 meta.json 이 없을 때다.
var ErrNoGraph = errors.New("no knowledge graph")

// GraphDir 는 /understand 의 데이터 디렉토리다. 레거시 .understand-anything/ 이 있으면 그것, 아니면 .ua/ (설계 2절).
func GraphDir(repo string) string {
	legacy := filepath.Join(repo, ".understand-anything")
	if st, err := os.Stat(legacy); err == nil && st.IsDir() {
		return legacy
	}
	return filepath.Join(repo, ".ua")
}

// GraphHash 는 meta.json 에 기록된 마지막 분석 커밋이다.
func GraphHash(repo string) (string, error) {
	b, err := os.ReadFile(filepath.Join(GraphDir(repo), "meta.json"))
	if errors.Is(err, os.ErrNotExist) {
		return "", ErrNoGraph
	}
	if err != nil {
		return "", err
	}
	var meta struct {
		GitCommitHash string `json:"gitCommitHash"`
	}
	if err := json.Unmarshal(b, &meta); err != nil {
		return "", fmt.Errorf("parse meta.json: %w", err)
	}
	if meta.GitCommitHash == "" {
		return "", ErrNoGraph
	}
	return meta.GitCommitHash, nil
}

// GraphDecision 은 설계 4절 ④ 의 세 갈래다.
type GraphDecision int

const (
	GraphUpToDate GraphDecision = iota // 해시 == HEAD: Claude 호출 안 함
	GraphRefresh                       // 증분 (또는 그래프가 아예 없어 스킬이 알아서 전체 분석)
	GraphFull                          // 해시가 레포에 없음: --full
)

func (d GraphDecision) String() string {
	switch d {
	case GraphUpToDate:
		return "up to date"
	case GraphFull:
		return "/understand --full"
	default:
		return "/understand"
	}
}

// DecideGraph 는 그래프 해시와 HEAD 를 비교해 무엇을 할지 정한다. /understand 는 해시가 같으면 사용자에게
// 되묻고 멈추므로 무인 실행에서는 여기서 먼저 걸러야 한다 (설계 2·9절).
func DecideGraph(ctx context.Context, repo, head string) (GraphDecision, string, error) {
	hash, err := GraphHash(repo)
	if errors.Is(err, ErrNoGraph) {
		return GraphRefresh, "", nil
	}
	if err != nil {
		return 0, "", err
	}
	switch {
	case hash == head:
		return GraphUpToDate, hash, nil
	case !CommitExists(ctx, repo, hash):
		return GraphFull, hash, nil
	default:
		return GraphRefresh, hash, nil
	}
}
