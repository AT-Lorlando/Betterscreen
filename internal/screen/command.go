package screen

import (
	"os/exec"
	"strconv"
)

// AttachCommand construit la commande d'attachement (handoff terminal).
// `-p <num>` positionne sur la fenêtre voulue à l'attachement.
func AttachCommand(s Session, w Window) *exec.Cmd {
	return exec.Command("screen", "-r", s.ID, "-p", strconv.Itoa(w.Num))
}

// killArgs : tue proprement une session (`-X quit` termine le démon).
func killArgs(s Session) []string {
	return []string{"-S", s.ID, "-X", "quit"}
}

// createArgs : crée une session détachée nommée.
func createArgs(name string) []string {
	return []string{"-dmS", name}
}

// selectArgs : sélectionne la fenêtre n d'une session (sans détacher).
func selectArgs(id string, n int) []string {
	return []string{"-S", id, "-X", "select", strconv.Itoa(n)}
}

// detachArgs : détache une session.
func detachArgs(id string) []string {
	return []string{"-S", id, "-X", "detach"}
}
