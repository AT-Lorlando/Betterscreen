package ui

import (
	"time"

	"betterscreen/internal/env"
	"betterscreen/internal/screen"

	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	api ScreenAPI

	sessions []screen.Session
	windows  []screen.Window
	details  map[int]screen.Detail

	focus      focusZone
	mode       uiMode
	selSession int
	selWindow  int
	input      string // saisie du nom lors de la création
	err        string

	loadedSessionID string // ID of the session whose windows are currently loaded

	inSession        bool
	currentSessionID string
	currentWindow    int
	handoff          Handoff

	width, height int
}

// New construit le modèle initial. Les options injectent le contexte et le handoff.
func New(api ScreenAPI, opts ...Option) Model {
	m := Model{
		api:           api,
		mode:          modeNormal,
		focus:         focusSessions,
		details:       map[int]screen.Detail{},
		currentWindow: -1,
	}
	for _, o := range opts {
		o(&m)
	}
	return m
}

// Handoff est le canal de saut inter-session (réel = handoff.Store, fake en test).
type Handoff interface {
	Write(sessionID string, window int) error
	ReadAndClear() (sessionID string, window int, ok bool)
}

// Option configure le Model à la construction.
type Option func(*Model)

// WithContext applique le contexte d'exécution (lanceur vs in-session).
func WithContext(ctx env.Context) Option {
	return func(m *Model) {
		m.inSession = ctx.InSession
		m.currentSessionID = ctx.SessionID
		m.currentWindow = ctx.Window
	}
}

// WithHandoff injecte le store de handoff.
func WithHandoff(h Handoff) Option {
	return func(m *Model) { m.handoff = h }
}

// messages internes
type sessionsMsg struct {
	sessions []screen.Session
	err      error
}
type windowsMsg struct {
	windows []screen.Window
	details map[int]screen.Detail
	err     error
}
type attachDoneMsg struct{ err error }
type tickMsg struct{}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadSessions(), tick())
}

// loadSessions récupère les sessions de façon asynchrone.
func (m Model) loadSessions() tea.Cmd {
	return func() tea.Msg {
		s, err := m.api.ListSessions()
		return sessionsMsg{sessions: s, err: err}
	}
}

// loadWindows récupère fenêtres + détails pour une session. En mode in-session,
// la fenêtre éphémère de BetterScreen (currentWindow) est retirée de la session courante.
func (m Model) loadWindows(s screen.Session) tea.Cmd {
	inSession := m.inSession
	curID := m.currentSessionID
	curWin := m.currentWindow
	return func() tea.Msg {
		w, err := m.api.ListWindows(s)
		if err != nil {
			return windowsMsg{err: err}
		}
		if inSession && s.ID == curID {
			w = filterWindow(w, curWin)
		}
		d, _ := m.api.InspectAll(s, w)
		return windowsMsg{windows: w, details: d}
	}
}

// filterWindow renvoie ws privé de la fenêtre de numéro num.
func filterWindow(ws []screen.Window, num int) []screen.Window {
	out := make([]screen.Window, 0, len(ws))
	for _, w := range ws {
		if w.Num == num {
			continue
		}
		out = append(out, w)
	}
	return out
}

// tick : polling de rafraîchissement (~2s).
func tick() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return tickMsg{} })
}

// currentSession renvoie la session sélectionnée, ou false si aucune.
func (m Model) currentSession() (screen.Session, bool) {
	if m.selSession < 0 || m.selSession >= len(m.sessions) {
		return screen.Session{}, false
	}
	return m.sessions[m.selSession], true
}

// selectedWin renvoie la fenêtre sélectionnée dans la liste UI, ou false.
func (m Model) selectedWin() (screen.Window, bool) {
	if m.selWindow < 0 || m.selWindow >= len(m.windows) {
		return screen.Window{}, false
	}
	return m.windows[m.selWindow], true
}
