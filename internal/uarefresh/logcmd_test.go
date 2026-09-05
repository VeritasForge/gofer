package uarefresh

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestShowLogPrintsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "2026-09-05.log")
	os.WriteFile(path, []byte("a\nb\n"), 0o644)
	var out bytes.Buffer
	if err := ShowLog(t.Context(), path, &out, false); err != nil || out.String() != "a\nb\n" {
		t.Fatalf("got %q %v", out.String(), err)
	}
}

func TestShowLogMissingFile(t *testing.T) {
	var out bytes.Buffer
	if err := ShowLog(t.Context(), filepath.Join(t.TempDir(), "none.log"), &out, false); err != nil || !strings.HasPrefix(out.String(), "no log yet:") {
		t.Fatalf("got %q %v", out.String(), err)
	}
}

func TestShowLogFollowAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "2026-09-05.log")
	os.WriteFile(path, []byte("a\n"), 0o644)
	ctx, cancel := context.WithCancel(t.Context())
	var mu sync.Mutex
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() { done <- ShowLog(ctx, path, lockedWriter{&mu, &out}, true) }()
	time.Sleep(200 * time.Millisecond)
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString("b\n")
	f.Close()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		s := out.String()
		mu.Unlock()
		if s == "a\nb\n" {
			cancel()
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	cancel()
	t.Fatalf("follow did not pick up the appended line; got %q", out.String())
}

type lockedWriter struct {
	mu *sync.Mutex
	w  *bytes.Buffer
}

func (l lockedWriter) Write(b []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(b)
}
