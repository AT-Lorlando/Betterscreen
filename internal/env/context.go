package env

import (
	"os"
	"strconv"
)

// Context décrit où tourne BetterScreen : lanceur (hors screen) ou in-session.
type Context struct {
	SessionID string // valeur de $STY (vide si hors session)
	Window    int    // valeur de $WINDOW, -1 si absente/invalide
	InSession bool   // true si $STY est défini
}

// Detect lit le contexte depuis l'environnement réel.
func Detect() Context { return contextFrom(os.Getenv) }

// contextFrom est le cœur testable : getenv est injecté.
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
