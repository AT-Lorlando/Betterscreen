package env

import (
	"os"
	"strconv"
)

// Context describes where BetterScreen runs: launcher (outside screen) or in-session.
type Context struct {
	SessionID string // value of $STY (empty if outside a session)
	Window    int    // value of $WINDOW, -1 if absent/invalid
	InSession bool   // true if $STY is set
}

// Detect reads the context from the real environment.
func Detect() Context { return contextFrom(os.Getenv) }

// contextFrom is the testable core: getenv is injected.
func contextFrom(getenv func(string) string) Context {
	sty := getenv("STY")
	if sty == "" {
		return Context{Window: -1}
	}
	win := -1
	if w, err := strconv.Atoi(getenv("WINDOW")); err == nil {
		win = w
	}
	return Context{SessionID: sty, Window: win, InSession: true}
}
