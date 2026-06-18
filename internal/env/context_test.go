package env

import "testing"

func TestContextLauncher(t *testing.T) {
	getenv := func(k string) string { return "" }
	c := contextFrom(getenv)
	if c.InSession {
		t.Error("empty STY must give InSession=false")
	}
	if c.Window != -1 {
		t.Errorf("Window = %d, want -1 when outside a session", c.Window)
	}
}

func TestContextInSession(t *testing.T) {
	env := map[string]string{"STY": "3378954.Astronix", "WINDOW": "2"}
	c := contextFrom(func(k string) string { return env[k] })
	if !c.InSession {
		t.Error("set STY must give InSession=true")
	}
	if c.SessionID != "3378954.Astronix" {
		t.Errorf("SessionID = %q", c.SessionID)
	}
	if c.Window != 2 {
		t.Errorf("Window = %d, want 2", c.Window)
	}
}

func TestContextInSessionNoWindow(t *testing.T) {
	env := map[string]string{"STY": "9.work"} // WINDOW absent
	c := contextFrom(func(k string) string { return env[k] })
	if !c.InSession {
		t.Error("set STY must give InSession=true")
	}
	if c.SessionID != "9.work" {
		t.Errorf("SessionID = %q, want 9.work", c.SessionID)
	}
	if c.Window != -1 {
		t.Errorf("Window = %d, want -1 when WINDOW absent/invalid", c.Window)
	}
}

func TestContextInSessionInvalidWindow(t *testing.T) {
	env := map[string]string{"STY": "9.work", "WINDOW": "abc"} // non-numeric
	c := contextFrom(func(k string) string { return env[k] })
	if c.Window != -1 {
		t.Errorf("Window = %d, want -1 when WINDOW is invalid", c.Window)
	}
}
