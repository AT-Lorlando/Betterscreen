package screen

import "testing"

// fakeFS simule /proc. Clés = chemins exacts.
type fakeFS struct {
	dirs  map[string][]string
	files map[string][]byte
	links map[string]string
}

func (f fakeFS) ReadDir(n string) ([]string, error) { return f.dirs[n], nil }
func (f fakeFS) ReadFile(n string) ([]byte, error)  { return f.files[n], nil }
func (f fakeFS) Readlink(n string) (string, error)  { return f.links[n], nil }

// stat: "pid (comm) state ppid ... " — on a besoin du champ 4 (ppid) et 22 (starttime).
// On remplit jusqu'au champ 22 avec des zéros.
func statLine(pid int, comm string, ppid, starttime int) string {
	s := itoa(pid) + " (" + comm + ") S " + itoa(ppid)
	for i := 5; i <= 21; i++ {
		s += " 0"
	}
	s += " " + itoa(starttime)
	return s
}
func itoa(i int) string { // évite d'importer strconv dans le test
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

func TestInspectAllReliable(t *testing.T) {
	// démon PID 100, deux shells enfants: pid 200 (starttime 10), pid 300 (starttime 20).
	fs := fakeFS{
		dirs: map[string][]string{"/proc": {"100", "200", "300"}},
		files: map[string][]byte{
			"/proc/100/stat": []byte(statLine(100, "screen", 1, 5)),
			"/proc/200/stat": []byte(statLine(200, "zsh", 100, 10)),
			"/proc/300/stat": []byte(statLine(300, "zsh", 100, 20)),
		},
		links: map[string]string{
			"/proc/200/cwd": "/home/chuya",
			"/proc/300/cwd": "/home/chuya/proj",
		},
	}
	s := Session{PID: 100, ID: "100.work"}
	windows := []Window{{Num: 0, Title: "zsh"}, {Num: 1, Title: "vim"}}

	got := InspectAll(fs, s, windows)
	if got[0].Pwd != "/home/chuya" {
		t.Errorf("window 0 pwd = %q, want /home/chuya", got[0].Pwd)
	}
	if got[1].Pwd != "/home/chuya/proj" {
		t.Errorf("window 1 pwd = %q, want /home/chuya/proj", got[1].Pwd)
	}
	if got[0].Proc != "zsh" {
		t.Errorf("window 0 proc = %q, want zsh", got[0].Proc)
	}
	if got[1].Proc != "zsh" {
		t.Errorf("window 1 proc = %q, want zsh", got[1].Proc)
	}
}

func TestInspectAllUnreliable(t *testing.T) {
	// 1 enfant mais 2 fenêtres → mapping non fiable → tout en "—".
	fs := fakeFS{
		dirs: map[string][]string{"/proc": {"100", "200"}},
		files: map[string][]byte{
			"/proc/100/stat": []byte(statLine(100, "screen", 1, 5)),
			"/proc/200/stat": []byte(statLine(200, "zsh", 100, 10)),
		},
		links: map[string]string{"/proc/200/cwd": "/home/chuya"},
	}
	s := Session{PID: 100, ID: "100.work"}
	windows := []Window{{Num: 0}, {Num: 1}}

	got := InspectAll(fs, s, windows)
	if got[0].Pwd != "—" || got[1].Pwd != "—" {
		t.Errorf("want both — , got %+v", got)
	}
}

func TestInspectAllForegroundWalk(t *testing.T) {
	// daemon 100, one shell 200 (ppid 100), shell has child 400 (vim, ppid 200).
	fs := fakeFS{
		dirs: map[string][]string{"/proc": {"100", "200", "400"}},
		files: map[string][]byte{
			"/proc/100/stat": []byte(statLine(100, "screen", 1, 5)),
			"/proc/200/stat": []byte(statLine(200, "zsh", 100, 10)),
			"/proc/400/stat": []byte(statLine(400, "vim", 200, 30)),
		},
		links: map[string]string{"/proc/200/cwd": "/home/chuya/proj"},
	}
	s := Session{PID: 100, ID: "100.work"}
	windows := []Window{{Num: 0, Title: "zsh"}}

	got := InspectAll(fs, s, windows)
	if got[0].Proc != "vim" {
		t.Errorf("window 0 proc = %q, want vim (foreground descendant)", got[0].Proc)
	}
	if got[0].Pwd != "/home/chuya/proj" {
		t.Errorf("window 0 pwd = %q, want /home/chuya/proj", got[0].Pwd)
	}
}
