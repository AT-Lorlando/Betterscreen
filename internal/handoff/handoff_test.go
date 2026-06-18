package handoff

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func fixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }

func TestWriteReadRoundTrip(t *testing.T) {
	now := time.Unix(1000, 0)
	path := filepath.Join(t.TempDir(), "handoff")
	s := newStore(path, fixedClock(now), 10*time.Second)

	if err := s.Write("3378954.Astronix", 2); err != nil {
		t.Fatalf("Write: %v", err)
	}
	id, win, ok := s.ReadAndClear()
	if !ok || id != "3378954.Astronix" || win != 2 {
		t.Errorf("got id=%q win=%d ok=%v", id, win, ok)
	}
}

func TestReadAndClearRemovesFile(t *testing.T) {
	now := time.Unix(1000, 0)
	path := filepath.Join(t.TempDir(), "handoff")
	s := newStore(path, fixedClock(now), 10*time.Second)
	_ = s.Write("A", -1)
	if _, _, ok := s.ReadAndClear(); !ok {
		t.Fatal("first read must succeed")
	}
	if _, _, ok := s.ReadAndClear(); ok {
		t.Error("second read must fail (file removed)")
	}
}

func TestStaleHandoffIgnored(t *testing.T) {
	path := filepath.Join(t.TempDir(), "handoff")
	// written at t=1000, read at t=1011 (>10s TTL)
	w := newStore(path, fixedClock(time.Unix(1000, 0)), 10*time.Second)
	_ = w.Write("A", 0)
	r := newStore(path, fixedClock(time.Unix(1011, 0)), 10*time.Second)
	if _, _, ok := r.ReadAndClear(); ok {
		t.Error("stale handoff must be ignored")
	}
}

func TestReadMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope")
	s := newStore(path, fixedClock(time.Unix(1000, 0)), 10*time.Second)
	if _, _, ok := s.ReadAndClear(); ok {
		t.Error("missing file must give ok=false")
	}
}

func TestReadBadWindowField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "handoff")
	// window field "x" is not an integer
	if err := os.WriteFile(path, []byte("A\tx\t1000\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := newStore(path, fixedClock(time.Unix(1000, 0)), 10*time.Second)
	if _, _, ok := s.ReadAndClear(); ok {
		t.Error("unreadable window field must give ok=false")
	}
}

func TestReadEmptySessionID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "handoff")
	if err := os.WriteFile(path, []byte("\t0\t1000\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := newStore(path, fixedClock(time.Unix(1000, 0)), 10*time.Second)
	if _, _, ok := s.ReadAndClear(); ok {
		t.Error("empty sessionID must give ok=false")
	}
}
