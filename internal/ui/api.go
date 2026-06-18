package ui

import (
	"os/exec"

	"betterscreen/internal/screen"
)

// ScreenAPI est le contrat métier injecté dans l'UI (réel = screen.Client,
// factice en test).
type ScreenAPI interface {
	ListSessions() ([]screen.Session, error)
	ListWindows(screen.Session) ([]screen.Window, error)
	InspectAll(screen.Session, []screen.Window) (map[int]screen.Detail, error)
	CreateSession(name string) error
	KillSession(screen.Session) error
	AttachCommand(screen.Session, screen.Window) *exec.Cmd
	SelectWindow(sessionID string, window int) error
	Detach(sessionID string) error
}
