package uarefresh

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	ua "gofer/internal/uarefresh"
)

func configCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Manage the ua-refresh config file"}
	cmd.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Write a config template to ~/.config/gofer/ua-refresh.toml (never overwrites)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			paths, err := ua.DefaultPaths()
			if err != nil {
				return err
			}
			if err := ua.WriteTemplate(paths.Config); err != nil {
				if errors.Is(err, os.ErrExist) {
					return fmt.Errorf("config file already exists: %s (edit it directly)", paths.Config)
				}
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %s — edit repos and webhook_url\n", paths.Config)
			return nil
		},
	})
	return cmd
}
