package uarefresh

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func writeMeta(t *testing.T, repo, dir, hash string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repo, dir), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"version":"2.9.4","gitCommitHash":"` + hash + `"}`
	if err := os.WriteFile(filepath.Join(repo, dir, "meta.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGraphDirPrefersLegacy(t *testing.T) {
	repo := t.TempDir()
	if got := GraphDir(repo); got != filepath.Join(repo, ".ua") {
		t.Errorf("default = %q", got)
	}
	os.MkdirAll(filepath.Join(repo, ".understand-anything"), 0o755)
	if got := GraphDir(repo); got != filepath.Join(repo, ".understand-anything") {
		t.Errorf("legacy = %q", got)
	}
}

func TestGraphHashMissing(t *testing.T) {
	_, err := GraphHash(t.TempDir())
	if !errors.Is(err, ErrNoGraph) {
		t.Fatalf("want ErrNoGraph, got %v", err)
	}
}

func TestDecideGraph(t *testing.T) {
	work, _ := newRepoWithOrigin(t, "main")
	ctx := t.Context()
	first, _ := Head(ctx, work)
	second := commitFile(t, work, "x.txt", "x")

	cases := []struct {
		name     string
		setup    func()
		want     GraphDecision
		wantHash string
	}{
		{"no graph", func() {}, GraphRefresh, ""},
		{"hash equals head", func() { writeMeta(t, work, ".ua", second) }, GraphUpToDate, second},
		{"hash behind head", func() { writeMeta(t, work, ".ua", first) }, GraphRefresh, first},
		{"hash not in repo", func() { writeMeta(t, work, ".ua", "0123456789abcdef0123456789abcdef01234567") }, GraphFull, "0123456789abcdef0123456789abcdef01234567"},
		{"legacy dir wins", func() { writeMeta(t, work, ".ua", first); writeMeta(t, work, ".understand-anything", second) }, GraphUpToDate, second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			os.RemoveAll(filepath.Join(work, ".ua"))
			os.RemoveAll(filepath.Join(work, ".understand-anything"))
			tc.setup()
			d, h, err := DecideGraph(ctx, work, second)
			if err != nil || d != tc.want || h != tc.wantHash {
				t.Fatalf("got (%v, %q, %v) want (%v, %q)", d, h, err, tc.want, tc.wantHash)
			}
		})
	}
}
