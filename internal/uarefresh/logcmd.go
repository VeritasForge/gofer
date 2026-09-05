package uarefresh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// ShowLog 는 로그 파일을 w 에 쓴다. follow 면 ctx 가 끝날 때까지 새로 붙는 내용을 계속 쓴다 (파일이 나중에 생겨도 된다).
func ShowLog(ctx context.Context, path string, w io.Writer, follow bool) error {
	var offset int64
	copyNew := func() error {
		f, err := os.Open(path)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return err
		}
		n, err := io.Copy(w, f)
		offset += n
		return err
	}
	if err := copyNew(); err != nil {
		return err
	}
	if !follow {
		if offset == 0 {
			fmt.Fprintf(w, "no log yet: %s\n", path)
		}
		return nil
	}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := copyNew(); err != nil {
				return err
			}
		}
	}
}
