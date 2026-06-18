package ui

import (
	"betterscreen/internal/screen"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil

	case sessionsMsg:
		return m.onSessions(msg)

	case windowsMsg:
		m.windows = msg.windows
		m.details = msg.details
		if m.selWindow >= len(m.windows) {
			m.selWindow = 0
		}
		if msg.err != nil {
			m.windows = nil
		}
		return m, nil

	case tickMsg:
		return m, tea.Batch(m.loadSessions(), tick())

	case attachDoneMsg:
		m.loadedSessionID = ""
		if msg.err != nil {
			m.err = "attach: " + msg.err.Error()
		}
		if m.handoff != nil {
			if id, win, ok := m.handoff.ReadAndClear(); ok {
				w := screen.Window{Num: 0}
				if win >= 0 {
					w.Num = win
				}
				cmd := m.api.AttachCommand(screen.Session{ID: id}, w)
				return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
					return attachDoneMsg{err: err}
				})
			}
		}
		return m, m.loadSessions()

	case tea.KeyMsg:
		return m.onKey(msg)
	}
	return m, nil
}

func (m Model) onSessions(msg sessionsMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.err = msg.err.Error()
		return m, nil
	}
	m.err = ""
	m.sessions = msg.sessions
	if m.selSession >= len(m.sessions) {
		m.selSession = max(0, len(m.sessions)-1)
	}
	if s, ok := m.currentSession(); ok {
		if s.ID != m.loadedSessionID {
			m.loadedSessionID = s.ID
			return m, m.loadWindows(s)
		}
		return m, nil
	}
	m.windows = nil
	m.loadedSessionID = ""
	return m, nil
}

func (m Model) onKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeConfirmKill:
		return m.onConfirmKill(msg)
	case modeNewSession:
		return m.onNewSession(msg)
	default:
		return m.onNormalKey(msg)
	}
}

func (m Model) onNormalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "j", "down":
		m = m.moveSelection(1)
		return m.maybeLoadWindows()
	case "k", "up":
		m = m.moveSelection(-1)
		return m.maybeLoadWindows()
	case "tab", "h", "l", "left", "right":
		if m.focus == focusSessions {
			m.focus = focusWindows
		} else {
			m.focus = focusSessions
		}
		return m, nil
	case "r":
		m.loadedSessionID = ""
		return m, m.loadSessions()
	case "n":
		m.mode = modeNewSession
		m.input = ""
		return m, nil
	case "d":
		if _, ok := m.currentSession(); ok {
			m.mode = modeConfirmKill
		}
		return m, nil
	case "enter":
		return m.onEnter()
	}
	return m, nil
}

func (m Model) moveSelection(delta int) Model {
	if m.focus == focusSessions {
		m.selSession = clamp(m.selSession+delta, 0, len(m.sessions)-1)
	} else {
		m.selWindow = clamp(m.selWindow+delta, 0, len(m.windows)-1)
	}
	return m
}

// maybeLoadWindows reloads the windows if the selected session changed.
func (m Model) maybeLoadWindows() (tea.Model, tea.Cmd) {
	if m.focus == focusSessions {
		if s, ok := m.currentSession(); ok && s.ID != m.loadedSessionID {
			m.loadedSessionID = s.ID
			return m, m.loadWindows(s)
		}
	}
	return m, nil
}

// onEnter decides the action based on the mode and the selected target.
func (m Model) onEnter() (tea.Model, tea.Cmd) {
	s, ok := m.currentSession()
	if !ok || s.State == screen.StateDead {
		return m, nil
	}
	w, wok := m.selectedWin() // note: method renamed in Task 4 (collision with the currentWindow field)
	win := 0
	if wok {
		win = w.Num
	}
	if m.inSession {
		return m.jump(s, win, wok)
	}
	cmd := m.api.AttachCommand(s, w)
	return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
		return attachDoneMsg{err: err}
	})
}

// jump handles in-session mode: select-in-place if it is the current session,
// otherwise handoff + detach.
func (m Model) jump(s screen.Session, win int, hasWindow bool) (tea.Model, tea.Cmd) {
	if s.ID == m.currentSessionID {
		if !hasWindow {
			return m, nil // no window selected: do nothing
		}
		_ = m.api.SelectWindow(m.currentSessionID, win)
		return m, tea.Quit
	}
	if m.handoff == nil {
		m.err = "handoff unavailable"
		return m, nil
	}
	if err := m.handoff.Write(s.ID, win); err != nil {
		m.err = "handoff: " + err.Error()
		return m, nil
	}
	_ = m.api.Detach(m.currentSessionID)
	return m, tea.Quit
}

func (m Model) onConfirmKill(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "y" {
		if s, ok := m.currentSession(); ok {
			_ = m.api.KillSession(s)
		}
		m.mode = modeNormal
		return m, m.loadSessions()
	}
	m.mode = modeNormal
	return m, nil
}

func (m Model) onNewSession(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		name := m.input
		m.mode = modeNormal
		m.input = ""
		if name != "" {
			_ = m.api.CreateSession(name)
		}
		return m, m.loadSessions()
	case tea.KeyEsc:
		m.mode = modeNormal
		m.input = ""
		return m, nil
	case tea.KeyBackspace:
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
		return m, nil
	case tea.KeyRunes:
		m.input += string(msg.Runes)
		return m, nil
	}
	return m, nil
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
