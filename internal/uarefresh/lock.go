package uarefresh

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// ErrAlreadyRunning 은 살아 있는 다른 실행이 잠금을 쥐고 있을 때다.
var ErrAlreadyRunning = errors.New("another ua-refresh run is in progress")

// AcquireLock 은 pid 를 담은 잠금 파일을 만든다. 파일이 있어도 그 pid 가 죽어 있으면 stale 로 보고 지운다 (설계 7절).
func AcquireLock(path string) (release func(), err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if err == nil {
			fmt.Fprintf(f, "%d\n", os.Getpid())
			f.Close()
			return func() { os.Remove(path) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, err
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil, rerr
		}
		pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
		if pid > 0 && processAlive(pid) {
			return nil, fmt.Errorf("%w (pid %d)", ErrAlreadyRunning, pid)
		}
		os.Remove(path) // stale — 다음 시도에서 새로 만든다
	}
	return nil, ErrAlreadyRunning
}

// processAlive 는 시그널 0 으로 생존을 확인한다. 권한 오류(EPERM)는 살아 있다는 뜻이다.
func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// LockHeld 는 잠금 파일이 지금 살아 있는 프로세스를 가리키는지 본다. 파일이 없거나 그 pid 가 죽어
// 있으면 held=false 다 — install 이 진행 중인 run 을 밟고 지나가지 않도록 미리 확인하는 데 쓴다.
func LockHeld(path string) (pid int, held bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	pid, _ = strconv.Atoi(strings.TrimSpace(string(b)))
	return pid, pid > 0 && processAlive(pid)
}
