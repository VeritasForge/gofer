package holiday

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	hol "gofer/internal/holiday"
)

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list [year]",
		Short: "Show the stored holidays of a year (default: this year)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			year := strconv.Itoa(time.Now().Year())
			if len(args) == 1 {
				if _, err := strconv.Atoi(args[0]); err != nil || len(args[0]) != 4 {
					return fmt.Errorf("year must be four digits, got %q", args[0])
				}
				year = args[0]
			}
			paths, err := hol.DefaultPaths()
			if err != nil {
				return err
			}
			f, err := hol.ReadFile(paths.Calendar())
			if err != nil {
				return fmt.Errorf("%w — run `gofer holiday sync` first", err)
			}
			n := 0
			for _, e := range f.Holidays {
				if strings.HasPrefix(e.Date, year) {
					fmt.Fprintf(cmd.OutOrStdout(), "%s  %s\n", e.Date, e.Name)
					n++
				}
			}
			if n == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no holidays stored for %s (list covers %s ~ %s)\n", year, f.Covers.From, f.Covers.To)
			}
			return nil
		},
	}
}
