package handoff

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Store persiste une cible de saut entre le mode in-session et le lanceur.
type Store struct {
	path string
	now  func() time.Time
	ttl  time.Duration
}

// New construit le Store réel (chemin XDG, horloge système, TTL 10 s).
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

// Write enregistre la cible (sessionID, window) horodatée.
func (s *Store) Write(sessionID string, window int) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	line := fmt.Sprintf("%s\t%d\t%d\n", sessionID, window, s.now().Unix())
	return os.WriteFile(s.path, []byte(line), 0o644)
}

// ReadAndClear lit la cible, supprime le fichier, et renvoie ok=false si absent,
// illisible, ou périmé (> TTL).
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

// Clear supprime tout handoff existant (purge au démarrage du lanceur).
func (s *Store) Clear() { _ = os.Remove(s.path) }
