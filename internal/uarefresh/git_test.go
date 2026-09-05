package uarefresh

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCurrentBranchAndDetached(t *testing.T) {
	work, _ := newRepoWithOrigin(t, "main")
	ctx := t.Context()
	if b, err := CurrentBranch(ctx, work); err != nil || b != "main" {
		t.Fatalf("branch = %q, %v", b, err)
	}
	run(t, work, "git", "checkout", "-q", "--detach")
	if b, err := CurrentBranch(ctx, work); err != nil || b != "" {
		t.Fatalf("detached branch = %q, %v", b, err)
	}
}

func TestHasTrackedChangesIgnoresUntracked(t *testing.T) {
	work, _ := newRepoWithOrigin(t, "main")
	ctx := t.Context()
	if dirty, _ := HasTrackedChanges(ctx, work); dirty {
		t.Fatal("clean repo reported dirty")
	}
	os.WriteFile(filepath.Join(work, "scratch.txt"), []byte("x"), 0o644)
	if dirty, _ := HasTrackedChanges(ctx, work); dirty {
		t.Fatal("untracked file should not count")
	}
	os.WriteFile(filepath.Join(work, "README.md"), []byte("changed"), 0o644)
	if dirty, _ := HasTrackedChanges(ctx, work); !dirty {
		t.Fatal("modified tracked file should count")
	}
}

func TestFetchCountAndFFMerge(t *testing.T) {
	work, origin := newRepoWithOrigin(t, "main")
	ctx := t.Context()
	pushFromClone(t, origin, "main", "a.txt", "a")
	want := pushFromClone(t, origin, "main", "b.txt", "b")
	if _, err := Fetch(ctx, work); err != nil {
		t.Fatal(err)
	}
	if n, err := CountCommits(ctx, work, "HEAD", "origin/main"); err != nil || n != 2 {
		t.Fatalf("behind = %d, %v", n, err)
	}
	if err := FFMerge(ctx, work, "main"); err != nil {
		t.Fatal(err)
	}
	if h, _ := Head(ctx, work); h != want {
		t.Fatalf("HEAD = %s want %s", h, want)
	}
}

func TestFFMergeFailsWhenLocalAhead(t *testing.T) {
	work, origin := newRepoWithOrigin(t, "main")
	ctx := t.Context()
	commitFile(t, work, "local.txt", "l")
	pushFromClone(t, origin, "main", "remote.txt", "r")
	if _, err := Fetch(ctx, work); err != nil {
		t.Fatal(err)
	}
	if err := FFMerge(ctx, work, "main"); err == nil {
		t.Fatal("diverged ff-merge should fail")
	}
	if n, _ := CountCommits(ctx, work, "origin/main", "HEAD"); n != 1 {
		t.Fatalf("ahead = %d", n)
	}
}

func TestCommitExists(t *testing.T) {
	work, _ := newRepoWithOrigin(t, "main")
	ctx := t.Context()
	h, _ := Head(ctx, work)
	if !CommitExists(ctx, work, h) {
		t.Fatal("HEAD should exist")
	}
	if CommitExists(ctx, work, "0000000000000000000000000000000000000000") {
		t.Fatal("zero hash should not exist")
	}
}
