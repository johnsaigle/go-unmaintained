package formatter

import (
	"os"
	"strings"
)

// Colorizer applies ANSI styling to strings. When disabled, all methods
// return their input unchanged.
type Colorizer struct {
	Enabled bool
}

// Red renders s in red.
func (c Colorizer) Red(s string) string { return c.wrap("\x1b[31m", s) }

// Yellow renders s in yellow.
func (c Colorizer) Yellow(s string) string { return c.wrap("\x1b[33m", s) }

// Green renders s in green.
func (c Colorizer) Green(s string) string { return c.wrap("\x1b[32m", s) }

// Bold renders s in bold.
func (c Colorizer) Bold(s string) string { return c.wrap("\x1b[1m", s) }

// Dim renders s in dimmed text.
func (c Colorizer) Dim(s string) string { return c.wrap("\x1b[2m", s) }

func (c Colorizer) wrap(code, s string) string {
	if !c.Enabled || s == "" {
		return s
	}
	return code + s + "\x1b[0m"
}

// ResolveColor determines whether colored output should be enabled.
// mode is "always", "auto", or "never". In auto mode, color is enabled only
// when f is a terminal and the NO_COLOR environment variable is not set.
func ResolveColor(mode string, f *os.File) bool {
	switch strings.ToLower(mode) {
	case "always":
		return true
	case "never":
		return false
	default: // auto
		if os.Getenv("NO_COLOR") != "" {
			return false
		}
		if f == nil {
			return false
		}
		info, err := f.Stat()
		if err != nil {
			return false
		}
		return info.Mode()&os.ModeCharDevice != 0
	}
}
