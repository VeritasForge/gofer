package main

import (
	"context"
	"os"

	"github.com/charmbracelet/fang"

	"gofer/cmd"
)

// version 은 `just build` 가 -ldflags 로 채운다.
var version = "dev"

func main() {
	if err := fang.Execute(context.Background(), cmd.Root(), fang.WithVersion(version)); err != nil {
		os.Exit(1)
	}
}
