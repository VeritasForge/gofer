package tui

import (
	"testing"
	"time"
)

func TestFormatElapsed(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{0, ""},
		{3 * time.Second, "3s"},
		{97 * time.Second, "1m37s"},
		{125 * time.Second, "2m05s"},
		{252 * time.Second, "4m12s"},
		{3725 * time.Second, "1h02m05s"},
	}
	for _, c := range cases {
		if got := FormatElapsed(c.d); got != c.want {
			t.Errorf("FormatElapsed(%s) = %q, want %q", c.d, got, c.want)
		}
	}
}
