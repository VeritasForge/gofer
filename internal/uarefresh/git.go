package uarefresh

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// git 은 `git -C dir args...` 를 실행한다. 실패하면 stderr 를 오류 문구에 담는다.
func git(ctx context.Context, dir string, args ...string) (stdout, stderr string, err error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err = cmd.Run()
	stdout, stderr = strings.TrimSpace(out.String()), strings.TrimSpace(errb.String())
	if err != nil {
		msg := stderr
		if msg == "" {
			msg = err.Error()
		}
		err = fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return stdout, stderr, err
}

// CurrentBranch 는 현재 브랜치 이름이다. detached HEAD 면 "" 를 돌려준다.
func CurrentBranch(ctx context.Context, dir string) (string, error) {
	out, _, err := git(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	if out == "HEAD" {
		return "", nil
	}
	return out, nil
}

// HasTrackedChanges 는 추적 파일에 미커밋 수정이 있는지 본다. 미추적 파일은 무시한다 (설계 4절).
func HasTrackedChanges(ctx context.Context, dir string) (bool, error) {
	out, _, err := git(ctx, dir, "status", "--porcelain", "--untracked-files=no")
	return out != "", err
}

// Fetch 는 `git fetch -ptf` 다. git 이 stderr 로 내는 진행 요약을 로그용으로 돌려준다.
func Fetch(ctx context.Context, dir string) (string, error) {
	_, stderr, err := git(ctx, dir, "fetch", "-ptf")
	return stderr, err
}

// Head 는 현재 HEAD 커밋 해시다.
func Head(ctx context.Context, dir string) (string, error) {
	out, _, err := git(ctx, dir, "rev-parse", "HEAD")
	return out, err
}

// CountCommits 는 from..to 사이 커밋 수다. 예: CountCommits(dir, "HEAD", "origin/main") = 뒤처진 수.
func CountCommits(ctx context.Context, dir, from, to string) (int, error) {
	out, _, err := git(ctx, dir, "rev-list", "--count", from+".."+to)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(out)
}

// FFMerge 는 origin/<trunk> 를 fast-forward 로만 합친다. 분기돼 있으면 실패한다.
func FFMerge(ctx context.Context, dir, trunk string) error {
	_, _, err := git(ctx, dir, "merge", "--ff-only", "origin/"+trunk)
	return err
}

// CommitExists 는 해시가 이 레포에 커밋으로 존재하는지 본다 (force-push 뒤엔 사라질 수 있다).
func CommitExists(ctx context.Context, dir, hash string) bool {
	_, _, err := git(ctx, dir, "cat-file", "-e", hash+"^{commit}")
	return err == nil
}
