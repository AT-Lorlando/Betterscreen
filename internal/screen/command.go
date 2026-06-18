package screen

import (
	"os/exec"
	"strconv"
)

// AttachCommand builds the attach command (terminal handoff).
// `-p <num>` positions on the desired window upon attach.
func AttachCommand(s Session, w Window) *exec.Cmd {
	return exec.Command("screen", "-r", s.ID, "-p", strconv.Itoa(w.Num))
}

// killArgs: cleanly kills a session (`-X quit` terminates the daemon).
func killArgs(s Session) []string {
	return []string{"-S", s.ID, "-X", "quit"}
}

// createArgs: creates a named detached session.
func createArgs(name string) []string {
	return []string{"-dmS", name}
}

// selectArgs: selects window n of a session (without detaching).
func selectArgs(id string, n int) []string {
	return []string{"-S", id, "-X", "select", strconv.Itoa(n)}
}

// detachArgs: detaches a session.
func detachArgs(id string) []string {
	return []string{"-S", id, "-X", "detach"}
}
