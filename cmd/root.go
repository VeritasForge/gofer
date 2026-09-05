// Package cmd 는 gofer 루트 커맨드다. 도구는 여기에 한 줄씩 등록한다.
package cmd

import (
	"github.com/spf13/cobra"

	"gofer/cmd/uarefresh"
)

// Root 는 `gofer` 루트 커맨드를 만든다.
func Root() *cobra.Command {
	root := &cobra.Command{
		Use:   "gofer",
		Short: "Personal CLI errand runner: gofer <tool> <action>",
	}
	root.AddCommand(uarefresh.Cmd())
	return root
}
