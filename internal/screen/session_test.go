package screen

import "testing"

func TestParseSessions(t *testing.T) {
	raw := "There are screens on:\n" +
		"\t3430249.thcon\t(18/06/2026 09:25:14)\t(Attached)\n" +
		"\t3378954.Astronix\t(18/06/2026 07:46:57)\t(Detached)\n" +
		"\t2705548.koya\t(17/06/2026 10:06:52)\t(Dead ???)\n" +
		"3 Sockets in /run/screen/S-chuya.\n"

	got := ParseSessions(raw)
	if len(got) != 3 {
		t.Fatalf("want 3 sessions, got %d", len(got))
	}
	if got[0].PID != 3430249 || got[0].Name != "thcon" || got[0].ID != "3430249.thcon" {
		t.Errorf("session 0 mal parsée: %+v", got[0])
	}
	if got[0].State != StateAttached {
		t.Errorf("session 0 state = %v, want Attached", got[0].State)
	}
	if got[1].State != StateDetached {
		t.Errorf("session 1 state = %v, want Detached", got[1].State)
	}
	if got[2].State != StateDead {
		t.Errorf("session 2 state = %v, want Dead", got[2].State)
	}
}

func TestParseSessionsEmpty(t *testing.T) {
	if got := ParseSessions("No Sockets found in /run/screen/S-chuya.\n"); len(got) != 0 {
		t.Errorf("want 0 sessions, got %d", len(got))
	}
}
