package uarefresh

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	ua "gofer/internal/uarefresh"
)

func installCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Register the daily launchd job (uses schedule.at from the config)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			paths, err := ua.DefaultPaths()
			if err != nil {
				return err
			}
			cfg, err := ua.Load(paths.Config)
			if err != nil {
				return err
			}
			exe, err := os.Executable()
			if err != nil {
				return err
			}
			if resolved, err := filepath.EvalSymlinks(exe); err == nil {
				exe = resolved
			}
			// install 은 launchd 작업을 내렸다 올린다 — 지금 run 이 돌고 있으면 그 claude 가 SIGTERM 을 맞는다.
			if pid, held := ua.LockHeld(paths.Lock()); held {
				return fmt.Errorf("a ua-refresh run is in progress (pid %d); retry when it finishes", pid)
			}
			if err := ua.Install(cmd.Context(), paths, cfg.Schedule.At, exe); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "installed %s: daily at %s\n  program: %s\n  plist:   %s\nre-run install if you move the gofer binary or change schedule.at\n",
				ua.LaunchdLabel, cfg.Schedule.At, exe, paths.Plist)
			return nil
		},
	}
}

func uninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the launchd job",
		RunE: func(cmd *cobra.Command, _ []string) error {
			paths, err := ua.DefaultPaths()
			if err != nil {
				return err
			}
			if err := ua.Uninstall(cmd.Context(), paths); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", ua.LaunchdLabel)
			return nil
		},
	}
}
