package handoff

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Store persists a jump target between in-session mode and the launcher.
type Store struct {
	path string
	now  func() time.Time
	ttl  time.Duration
}

// New builds the real Store (XDG path, system clock, 10s TTL).
func New() *Store {
	return newStore(defaultPath(), time.Now, 10*time.Second)
}

func newStore(path string, now func() time.Time, ttl time.Duration) *Store {
	return &Store{path: path, now: now, ttl: ttl}
}

func defaultPath() string {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".cache")
	}
	return filepath.Join(dir, "betterscreen", "handoff")
}

// Write records the (sessionID, window) target with a timestamp.
func (s *Store) Write(sessionID string, window int) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	line := fmt.Sprintf("%s\t%d\t%d\n", sessionID, window, s.now().Unix())
	return os.WriteFile(s.path, []byte(line), 0o644)
}

// ReadAndClear reads the target, removes the file, and returns ok=false if it is
// missing, unreadable, or stale (> TTL).
func (s *Store) ReadAndClear() (string, int, bool) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return "", -1, false
	}
	_ = os.Remove(s.path)
	fields := strings.Split(strings.TrimSpace(string(data)), "\t")
	if len(fields) != 3 || fields[0] == "" {
		return "", -1, false
	}
	win, err := strconv.Atoi(fields[1])
	if err != nil {
		return "", -1, false
	}
	ts, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil {
		return "", -1, false
	}
	if s.now().Sub(time.Unix(ts, 0)) > s.ttl {
		return "", -1, false
	}
	return fields[0], win, true
}

// Clear removes any existing handoff (purge at launcher startup).
func (s *Store) Clear() { _ = os.Remove(s.path) }
