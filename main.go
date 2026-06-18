package main

import (
	"fmt"
	"os"
	"os/exec"

	"betterscreen/internal/env"
	"betterscreen/internal/handoff"
	"betterscreen/internal/screen"
	"betterscreen/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if _, err := exec.LookPath("screen"); err != nil {
		fmt.Fprintln(os.Stderr, "Erreur : 'screen' introuvable dans le PATH. Installe GNU Screen.")
		os.Exit(1)
	}

	ctx := env.Detect()
	ho := handoff.New()
	if !ctx.InSession {
		ho.Clear() // purge un handoff périmé au démarrage du lanceur
	}

	model := ui.New(screen.NewClient(), ui.WithContext(ctx), ui.WithHandoff(ho))
	if _, err := tea.NewProgram(model, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Erreur :", err)
		os.Exit(1)
	}
}
