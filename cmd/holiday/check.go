package holiday

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	hol "gofer/internal/holiday"
)

func checkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "check [YYYY-MM-DD]",
		Short: "Tell whether a date is a workday (default: today)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			when := time.Now()
			if len(args) == 1 {
				parsed, err := time.ParseInLocation("2006-01-02", args[0], time.Local)
				if err != nil {
					return fmt.Errorf("date must be YYYY-MM-DD, got %q", args[0])
				}
				when = parsed
			}
			paths, err := hol.DefaultPaths()
			if err != nil {
				return err
			}
			cal, warning, err := hol.Open(paths, when)
			if err != nil {
				return err
			}
			if warning != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "warning: "+warning)
			}
			if reason, off := cal.Holiday(when); off {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  holiday — %s\n", when.Format("2006-01-02 (Mon)"), reason)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s  workday\n", when.Format("2006-01-02 (Mon)"))
			return nil
		},
	}
}
