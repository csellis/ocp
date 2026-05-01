package main

// ANSI escape sequences for the TUI. Kept tiny on purpose: only the
// styles the home and respond views actually use. If we add a theme
// system later it lives here.
const (
	ansiReset    = "\033[0m"
	ansiBold     = "\033[1m"
	ansiDim      = "\033[2m"
	ansiCyan     = "\033[36m"
	ansiBoldCyan = "\033[1;36m"
)

// colorize wraps s in ANSI codes when on, returns s unchanged when off.
// Single chokepoint so tests pass on=false to assert plain text.
func colorize(s, code string, on bool) string {
	if !on {
		return s
	}
	return code + s + ansiReset
}

// stylize is the convenience for the common shapes used in the TUI.
type stylist struct{ on bool }

func (s stylist) bold(t string) string     { return colorize(t, ansiBold, s.on) }
func (s stylist) dim(t string) string      { return colorize(t, ansiDim, s.on) }
func (s stylist) cyan(t string) string     { return colorize(t, ansiCyan, s.on) }
func (s stylist) boldCyan(t string) string { return colorize(t, ansiBoldCyan, s.on) }
