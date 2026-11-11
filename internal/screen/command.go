package screen

import (
	"os/exec"
	"strconv"
)

// AttachCommand builds the attach command (terminal handoff).
// `-d -r` detaches the session from any other terminal first, so a session
// still attached elsewhere can be taken over instead of failing with
// "There is a screen on ... (Attached)". `-p <num>` positions on the desired
// window upon attach; a negative Num keeps the session's current window.
func AttachCommand(s Session, w Window) *exec.Cmd {
	return exec.Command("screen", withWindow([]string{"-d", "-r", s.ID}, w.Num)...)
}

// ShareCommand builds a multi-display attach (`screen -x`): the session is
// joined without detaching its other displays. `-p <num>` selects the window;
// a negative window keeps the session's current one.
func ShareCommand(s Session, window int) *exec.Cmd {
	return exec.Command("screen", withWindow([]string{"-x", s.ID}, window)...)
}

// withWindow appends `-p <window>` unless window is negative.
func withWindow(args []string, window int) []string {
	if window < 0 {
		return args
	}
	return append(args, "-p", strconv.Itoa(window))
}

// killArgs: cleanly kills a session (`-X quit` terminates the daemon).
func killArgs(s Session) []string {
	return []string{"-S", s.ID, "-X", "quit"}
}

// createArgs: creates a named detached session.
// `-T screen-256color` forces a 256-color window TERM: screen otherwise imposes
// its own default ("screen", 8 colors) regardless of the parent environment,
// which leaves color-gated shell prompts uncolored.
func createArgs(name string) []string {
	return []string{"-T", "screen-256color", "-dmS", name}
}

// selectArgs: selects window n of a session (without detaching).
func selectArgs(id string, n int) []string {
	return []string{"-S", id, "-X", "select", strconv.Itoa(n)}
}

// detachArgs: detaches a session.
func detachArgs(id string) []string {
	return []string{"-S", id, "-X", "detach"}
}
