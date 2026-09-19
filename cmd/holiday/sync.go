package holiday

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	hol "gofer/internal/holiday"
)

func syncCmd() *cobra.Command {
	var initConfig bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Download the holiday calendar and store it locally",
		RunE: func(cmd *cobra.Command, _ []string) error {
			paths, err := hol.DefaultPaths()
			if err != nil {
				return err
			}
			if initConfig {
				return writeConfigTemplate(cmd, paths.Config)
			}
			f, err := hol.Sync(cmd.Context(), paths, time.Now())
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s ~ %s · %d holidays stored\n  %s\n",
				f.Covers.From, f.Covers.To, len(f.Holidays), paths.Calendar())
			return nil
		},
	}
	cmd.Flags().BoolVar(&initConfig, "init-config", false, "write the config template instead of downloading")
	return cmd
}

// writeConfigTemplate 은 설정 템플릿을 0600 으로 새로 만든다. 이미 있으면 건드리지 않는다.
func writeConfigTemplate(cmd *cobra.Command, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(hol.ConfigTemplate); err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", path)
	return nil
}
