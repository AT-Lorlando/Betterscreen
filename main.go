package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"betterscreen/internal/env"
	"betterscreen/internal/handoff"
	"betterscreen/internal/screen"
	"betterscreen/internal/serve"
	"betterscreen/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if _, err := exec.LookPath("screen"); err != nil {
		fmt.Fprintln(os.Stderr, "Error: 'screen' not found in PATH. Install GNU Screen.")
		os.Exit(1)
	}

	if len(os.Args) > 1 && os.Args[1] == "serve" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		if err := serve.Run(ctx, os.Args[2:], os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		return
	}

	ctx := env.Detect()
	ho := handoff.New()
	if !ctx.InSession {
		ho.Clear() // purge a stale handoff at launcher startup
	}

	model := ui.New(screen.NewClient(), ui.WithContext(ctx), ui.WithHandoff(ho))
	if _, err := tea.NewProgram(model, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
