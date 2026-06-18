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
	input      string // name input during creation
	err        string

	loadedSessionID string // ID of the session whose windows are currently loaded

	inSession        bool
	currentSessionID string
	currentWindow    int
	handoff          Handoff

	width, height int
}

// New builds the initial model. Options inject the context and the handoff.
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

// Handoff is the inter-session jump channel (real = handoff.Store, fake in tests).
type Handoff interface {
	Write(sessionID string, window int) error
	ReadAndClear() (sessionID string, window int, ok bool)
}

// Option configures the Model at construction time.
type Option func(*Model)

// WithContext applies the execution context (launcher vs in-session).
func WithContext(ctx env.Context) Option {
	return func(m *Model) {
		m.inSession = ctx.InSession
		m.currentSessionID = ctx.SessionID
		m.currentWindow = ctx.Window
	}
}

// WithHandoff injects the handoff store.
func WithHandoff(h Handoff) Option {
	return func(m *Model) { m.handoff = h }
}

// internal messages
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

// loadSessions fetches the sessions asynchronously.
func (m Model) loadSessions() tea.Cmd {
	return func() tea.Msg {
		s, err := m.api.ListSessions()
		return sessionsMsg{sessions: s, err: err}
	}
}

// loadWindows fetches windows + details for a session. In in-session mode,
// BetterScreen's ephemeral window (currentWindow) is removed from the current session.
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

// filterWindow returns ws without the window numbered num.
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

// tick: refresh polling (~2s).
func tick() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return tickMsg{} })
}

// currentSession returns the selected session, or false if none.
func (m Model) currentSession() (screen.Session, bool) {
	if m.selSession < 0 || m.selSession >= len(m.sessions) {
		return screen.Session{}, false
	}
	return m.sessions[m.selSession], true
}

// selectedWin returns the window selected in the UI list, or false.
func (m Model) selectedWin() (screen.Window, bool) {
	if m.selWindow < 0 || m.selWindow >= len(m.windows) {
		return screen.Window{}, false
	}
	return m.windows[m.selWindow], true
}
