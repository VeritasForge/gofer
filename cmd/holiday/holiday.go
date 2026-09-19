// Package holiday 는 `gofer holiday` 서브커맨드 트리다. 플래그 파싱만 하고
// 로직은 internal/holiday 에 둔다.
package holiday

import "github.com/spf13/cobra"

// Cmd 는 `holiday` 부모 커맨드를 만든다.
func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "holiday",
		Short: "Keep a local holiday calendar and tell workdays from days off",
		Long: `Downloads a public holiday calendar once and stores it locally, so other tools can ask
"is today a workday?" without touching the network. Weekends need no stored list.`,
	}
	cmd.AddCommand(syncCmd(), listCmd(), checkCmd())
	return cmd
}
