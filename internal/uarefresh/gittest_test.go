package uarefresh

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// isolateGit 은 사용자 전역 git 설정(서명 등)이 테스트에 끼어들지 않게 한다.
func isolateGit(t *testing.T) {
	t.Helper()
	cfg := filepath.Join(t.TempDir(), "gitconfig")
	body := "[user]\n\tname = test\n\temail = test@example.com\n[commit]\n\tgpgsign = false\n[init]\n\tdefaultBranch = main\n"
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_CONFIG_GLOBAL", cfg)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

// run 은 외부 명령을 dir 에서 실행하고 출력을 돌려준다. 실패하면 테스트를 멈춘다.
func run(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// newRepoWithOrigin 은 bare origin 과 그것을 추적하는 작업 트리 example-api 를 만든다. 첫 커밋이 push 돼 있다.
func newRepoWithOrigin(t *testing.T, trunk string) (work, origin string) {
	t.Helper()
	isolateGit(t)
	base := t.TempDir()
	origin = filepath.Join(base, "origin.git")
	work = filepath.Join(base, "example-api")
	run(t, base, "git", "init", "-q", "--bare", "-b", trunk, origin)
	run(t, base, "git", "init", "-q", "-b", trunk, work)
	run(t, work, "git", "remote", "add", "origin", origin)
	commitFile(t, work, "README.md", "hello\n")
	run(t, work, "git", "push", "-q", "-u", "origin", trunk)
	return work, origin
}

// commitFile 은 파일을 쓰고 커밋한 뒤 새 HEAD 해시를 돌려준다.
func commitFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, dir, "git", "add", name)
	run(t, dir, "git", "commit", "-q", "-m", "add "+name)
	return run(t, dir, "git", "rev-parse", "HEAD")
}

// pushFromClone 은 별도 clone 에서 커밋해 origin 을 앞서게 만든다. 돌려주는 값은 origin 의 새 HEAD.
func pushFromClone(t *testing.T, origin, trunk, name, content string) string {
	t.Helper()
	other := filepath.Join(t.TempDir(), "other")
	run(t, filepath.Dir(other), "git", "clone", "-q", "-b", trunk, origin, other)
	hash := commitFile(t, other, name, content)
	run(t, other, "git", "push", "-q", "origin", trunk)
	return hash
}
