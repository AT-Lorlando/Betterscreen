package screen

import (
	"os/exec"
)

// Client exécute réellement les commandes screen.
type Client struct{}

func NewClient() Client { return Client{} }

func (Client) ListSessions() ([]Session, error) {
	// `screen -ls` renvoie un code de sortie non-nul quand il y a des sessions ;
	// on ignore l'erreur de code et on parse toujours la sortie.
	out, _ := exec.Command("screen", "-ls").CombinedOutput()
	return ParseSessions(string(out)), nil
}

func (Client) ListWindows(s Session) ([]Window, error) {
	out, err := exec.Command("screen", "-S", s.ID, "-Q", "windows").CombinedOutput()
	if err != nil {
		return nil, err
	}
	return ParseWindows(string(out)), nil
}

func (Client) InspectAll(s Session, windows []Window) (map[int]Detail, error) {
	return InspectAll(OSProcFS(), s, windows), nil
}

func (Client) CreateSession(name string) error {
	return exec.Command("screen", createArgs(name)...).Run()
}

func (Client) KillSession(s Session) error {
	return exec.Command("screen", killArgs(s)...).Run()
}

func (Client) AttachCommand(s Session, w Window) *exec.Cmd {
	return AttachCommand(s, w)
}

func (Client) SelectWindow(id string, n int) error {
	return exec.Command("screen", selectArgs(id, n)...).Run()
}

func (Client) Detach(id string) error {
	return exec.Command("screen", detachArgs(id)...).Run()
}
