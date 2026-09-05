package uarefresh

import (
	"os"

	"github.com/spf13/cobra"

	"gofer/internal/tui"
	ua "gofer/internal/uarefresh"
)

func runCmd() *cobra.Command {
	var dryRun bool
	var only string
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Fetch, ff-merge and refresh the knowledge graph of every configured repo",
		Long: `Processes the repos in ~/.config/gofer/ua-refresh.toml in order: guard (root worktree on trunk,
no uncommitted tracked changes) → git fetch -ptf → git merge --ff-only → run /understand only when
the graph hash differs from HEAD. Sends one Slack DM at the end. Exit code 1 if any repo was skipped or failed.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			paths, err := ua.DefaultPaths()
			if err != nil {
				return err
			}
			return ua.RunCommand(cmd.Context(), ua.CommandOptions{
				Paths: paths, DryRun: dryRun, Only: only,
				Stdout: os.Stdout, TTY: tui.IsTerminal(os.Stdout),
			})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate config and print what would happen; touches neither git nor claude")
	cmd.Flags().StringVar(&only, "only", "", "process only the repo with this directory name")
	return cmd
}
