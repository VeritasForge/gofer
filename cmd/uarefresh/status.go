package uarefresh

import (
	"fmt"

	"github.com/spf13/cobra"

	ua "gofer/internal/uarefresh"
)

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show launchd registration, last run summary and per-repo graph freshness",
		RunE: func(cmd *cobra.Command, _ []string) error {
			paths, err := ua.DefaultPaths()
			if err != nil {
				return err
			}
			cfg, err := ua.Load(paths.Config)
			if err != nil {
				return err
			}
			installed, err := ua.Installed(cmd.Context())
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), ua.StatusText(cmd.Context(), paths, cfg, installed))
			return nil
		},
	}
}
