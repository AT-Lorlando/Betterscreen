package screen

import (
	"os/exec"
)

// Client actually runs the screen commands.
type Client struct{}

func NewClient() Client { return Client{} }

func (Client) ListSessions() ([]Session, error) {
	// `screen -ls` returns a non-zero exit code when there are sessions;
	// we ignore the exit-code error and always parse the output.
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
