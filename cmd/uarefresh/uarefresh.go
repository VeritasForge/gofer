// Package uarefresh 는 `gofer ua-refresh` 서브커맨드 트리다. 플래그 파싱만 하고
// 로직은 internal/uarefresh 에 둔다.
package uarefresh

import "github.com/spf13/cobra"

// Cmd 는 `ua-refresh` 부모 커맨드를 만든다. 서브커맨드는 각 파일에서 하나씩 붙인다.
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ua-refresh",
		Short: "Refresh Understand-Anything knowledge graphs of your repos every morning",
	}
	cmd.AddCommand(configCmd())
	cmd.AddCommand(runCmd())
	cmd.AddCommand(installCmd(), uninstallCmd())
	cmd.AddCommand(statusCmd(), logCmd())
	return cmd
}
