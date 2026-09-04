package uarefresh

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestAcquireLockBlocksSecondRunner(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "run.lock")
	release, err := AcquireLock(path)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if strings.TrimSpace(string(b)) != strconv.Itoa(os.Getpid()) {
		t.Errorf("lock should hold our pid, got %q", b)
	}
	if _, err := AcquireLock(path); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second acquire: want ErrAlreadyRunning, got %v", err)
	}
	release()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Error("release should remove the lock file")
	}
}

func TestLockHeld(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.lock")
	if _, held := LockHeld(path); held {
		t.Error("missing lock file should not be held")
	}

	os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())+"\n"), 0o644)
	if pid, held := LockHeld(path); !held || pid != os.Getpid() {
		t.Errorf("own pid: got (%d, %v), want (%d, true)", pid, held, os.Getpid())
	}

	cmd := exec.Command("true") // 이미 끝난 프로세스의 pid 를 얻는다
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(path, []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0o644)
	if _, held := LockHeld(path); held {
		t.Error("dead pid should not be held")
	}
}

func TestAcquireLockRemovesStaleLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.lock")
	cmd := exec.Command("true") // 이미 끝난 프로세스의 pid 를 얻는다
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(path, []byte(strconv.Itoa(cmd.Process.Pid)+"\n"), 0o644)
	release, err := AcquireLock(path)
	if err != nil {
		t.Fatalf("stale lock should be replaced, got %v", err)
	}
	defer release()
	b, _ := os.ReadFile(path)
	if strings.TrimSpace(string(b)) != strconv.Itoa(os.Getpid()) {
		t.Errorf("lock should now hold our pid, got %q", b)
	}
}
