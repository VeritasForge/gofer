package main

import (
	"context"
	"os"
	"syscall"

	"github.com/charmbracelet/fang"

	"gofer/cmd"
)

// version 은 `just build` 가 -ldflags 로 채운다.
var version = "dev"

func main() {
	err := fang.Execute(context.Background(), cmd.Root(),
		fang.WithVersion(version),
		fang.WithNotifySignal(os.Interrupt, syscall.SIGTERM),
	)
	if err != nil {
		os.Exit(1)
	}
}
