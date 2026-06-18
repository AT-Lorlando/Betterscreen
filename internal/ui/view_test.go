package ui

import (
	"strings"
	"testing"

	"betterscreen/internal/env"
	"betterscreen/internal/screen"
)

func TestViewShowsSessionsAndWindows(t *testing.T) {
	m := New(&fakeAPI{})
	m.width, m.height = 80, 24
	m.sessions = []screen.Session{{ID: "1.work", Name: "work", State: screen.StateDetached}}
	m.windows = []screen.Window{{Num: 0, Title: "zsh"}}
	m.details = map[int]screen.Detail{0: {Pwd: "/home/chuya", Proc: "zsh"}}

	out := m.View()
	for _, want := range []string{"work", "zsh", "/home/chuya"} {
		if !strings.Contains(out, want) {
			t.Errorf("View() does not contain %q", want)
		}
	}
}

func TestViewEmptyState(t *testing.T) {
	m := New(&fakeAPI{})
	m.width, m.height = 80, 24
	if !strings.Contains(m.View(), "No sessions") {
		t.Error("View() must show the empty state")
	}
}

func TestViewConfirmKillPrompt(t *testing.T) {
	m := New(&fakeAPI{})
	m.width, m.height = 80, 24
	m.sessions = []screen.Session{{ID: "1.work", Name: "work"}}
	m.mode = modeConfirmKill
	if !strings.Contains(m.View(), "work") || !strings.Contains(strings.ToLower(m.View()), "kill") {
		t.Error("View() must show the kill confirmation")
	}
}

func TestViewNewSessionPrompt(t *testing.T) {
	m := New(&fakeAPI{})
	m.width, m.height = 80, 24
	m.sessions = []screen.Session{{ID: "1.work", Name: "work"}}
	m.mode = modeNewSession
	m.input = "demo"
	out := m.View()
	if !strings.Contains(out, "New session name") || !strings.Contains(out, "demo") {
		t.Error("View() must show the new-session prompt with the input")
	}
}

func TestViewMarksCurrentSession(t *testing.T) {
	m := New(&fakeAPI{}, WithContext(env.Context{SessionID: "1.work", InSession: true}))
	m.width, m.height = 80, 24
	m.sessions = []screen.Session{
		{ID: "1.work", Name: "work", State: screen.StateAttached},
		{ID: "2.x", Name: "x", State: screen.StateDetached},
	}
	if !strings.Contains(m.View(), "●") {
		t.Error("expected the ● marker on the current session")
	}
}
