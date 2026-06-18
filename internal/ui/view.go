package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	activeBorder = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("69")).Padding(0, 1)
	idleBorder   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(0, 1)
	selectedRow  = lipgloss.NewStyle().Foreground(lipgloss.Color("213")).Bold(true)
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

func (m Model) View() string {
	if len(m.sessions) == 0 {
		return "\n  Aucune session — [n] pour en créer, [q] pour quitter\n"
	}

	colW := m.width/2 - 4
	if colW < 10 {
		colW = 10
	}

	left := m.renderSessions(colW)
	right := m.renderWindows(colW)
	panels := lipgloss.JoinHorizontal(lipgloss.Top,
		boxFor(m.focus == focusSessions, "Sessions", left, colW),
		boxFor(m.focus == focusWindows, "Windows", right, colW),
	)

	detailW := m.width - 4
	if detailW < 1 {
		detailW = 1
	}
	detail := idleBorder.Width(detailW).Render(m.renderDetail())
	help := helpStyle.Render(" [↑↓] nav  [tab] panneau  [↵] attach  [n]ew  [d]el  [r] refresh  [q]uit")

	out := panels + "\n" + detail + "\n" + help
	if m.mode == modeConfirmKill {
		out += "\n" + m.renderConfirmKill()
	}
	if m.mode == modeNewSession {
		out += "\n  Nouveau nom de session: " + m.input + "▌"
	}
	if m.err != "" {
		out += "\n  ⚠ " + m.err
	}
	return out
}

func boxFor(active bool, title, body string, w int) string {
	style := idleBorder
	if active {
		style = activeBorder
	}
	return style.Width(w).Render(title + "\n" + body)
}

func (m Model) renderSessions(w int) string {
	var b strings.Builder
	for i, s := range m.sessions {
		marker := ""
		if m.inSession && s.ID == m.currentSessionID {
			marker = "● "
		}
		line := fmt.Sprintf("%s%s [%s]", marker, s.Name, s.State)
		b.WriteString(rowMarker(i == m.selSession) + truncate(line, w) + "\n")
	}
	return b.String()
}

func (m Model) renderWindows(w int) string {
	if len(m.windows) == 0 {
		return "(aucune fenêtre)"
	}
	var b strings.Builder
	for i, win := range m.windows {
		line := fmt.Sprintf("%d %s", win.Num, win.Title)
		b.WriteString(rowMarker(i == m.selWindow) + truncate(line, w) + "\n")
	}
	return b.String()
}

func (m Model) renderDetail() string {
	w, ok := m.selectedWin()
	if !ok {
		return "pwd: —   proc: —"
	}
	d := m.details[w.Num]
	if d.Pwd == "" {
		d.Pwd = "—"
	}
	if d.Proc == "" {
		d.Proc = "—"
	}
	return fmt.Sprintf("pwd: %s   proc: %s", d.Pwd, d.Proc)
}

func (m Model) renderConfirmKill() string {
	if s, ok := m.currentSession(); ok {
		return fmt.Sprintf("  Tuer la session %q ? [y/N]", s.Name)
	}
	return ""
}

func rowMarker(selected bool) string {
	if selected {
		return selectedRow.Render("> ")
	}
	return "  "
}

func truncate(s string, w int) string {
	runes := []rune(s)
	if w > 3 && len(runes) > w {
		return string(runes[:w-1]) + "…"
	}
	return s
}
