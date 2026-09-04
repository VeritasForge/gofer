package uarefresh

import (
	"time"

	"github.com/spf13/cobra"

	ua "gofer/internal/uarefresh"
)

func logCmd() *cobra.Command {
	var follow bool
	cmd := &cobra.Command{
		Use:   "log",
		Short: "Print today's log (--follow to keep watching)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			paths, err := ua.DefaultPaths()
			if err != nil {
				return err
			}
			return ua.ShowLog(cmd.Context(), paths.DailyLog(time.Now()), cmd.OutOrStdout(), follow)
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "keep printing as the log grows (ctrl+c to stop)")
	return cmd
}
