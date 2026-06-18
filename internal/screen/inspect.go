package screen

import (
	"os"
	"sort"
	"strconv"
	"strings"
)

const unknown = "—"

// ProcFS abstrait l'accès à /proc pour la testabilité.
type ProcFS interface {
	ReadDir(name string) ([]string, error)
	ReadFile(name string) ([]byte, error)
	Readlink(name string) (string, error)
}

type osProcFS struct{}

func (osProcFS) ReadDir(name string) ([]string, error) {
	entries, err := os.ReadDir(name)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names, nil
}
func (osProcFS) ReadFile(name string) ([]byte, error) { return os.ReadFile(name) }
func (osProcFS) Readlink(name string) (string, error) { return os.Readlink(name) }

// OSProcFS renvoie l'implémentation réelle basée sur /proc.
func OSProcFS() ProcFS { return osProcFS{} }

type proc struct {
	pid       int
	comm      string
	ppid      int
	starttime int
}

// statFields parse /proc/<pid>/stat. comm peut contenir des espaces/parenthèses,
// on isole donc la dernière ")" avant de découper le reste.
func statFields(data []byte) (proc, bool) {
	s := string(data)
	open := strings.IndexByte(s, '(')
	close := strings.LastIndexByte(s, ')')
	if open < 0 || close < 0 || close < open {
		return proc{}, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(s[:open]))
	if err != nil {
		return proc{}, false
	}
	comm := s[open+1 : close]
	rest := strings.Fields(s[close+1:])
	// rest[0]=state, rest[1]=ppid, ... rest[19]=starttime (champ 22 global).
	if len(rest) < 20 {
		return proc{}, false
	}
	ppid, _ := strconv.Atoi(rest[1])
	start, _ := strconv.Atoi(rest[19])
	return proc{pid: pid, comm: comm, ppid: ppid, starttime: start}, true
}

func readProc(fs ProcFS, pid int) (proc, bool) {
	data, err := fs.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return proc{}, false
	}
	return statFields(data)
}

func allProcs(fs ProcFS) []proc {
	names, err := fs.ReadDir("/proc")
	if err != nil {
		return nil
	}
	var out []proc
	for _, n := range names {
		pid, err := strconv.Atoi(n)
		if err != nil {
			continue // ignore les entrées non numériques
		}
		if p, ok := readProc(fs, pid); ok {
			out = append(out, p)
		}
	}
	return out
}

// InspectAll associe best-effort chaque fenêtre à un pwd + process.
func InspectAll(fs ProcFS, s Session, windows []Window) map[int]Detail {
	out := make(map[int]Detail, len(windows))
	for _, w := range windows {
		out[w.Num] = Detail{Pwd: unknown, Proc: unknown}
	}

	procs := allProcs(fs)
	var children []proc
	for _, p := range procs {
		if p.ppid == s.PID {
			children = append(children, p)
		}
	}

	// Mapping fiable seulement si autant d'enfants que de fenêtres.
	if len(children) != len(windows) || len(windows) == 0 {
		return out
	}

	sort.Slice(children, func(i, j int) bool { return children[i].starttime < children[j].starttime })
	sorted := append([]Window(nil), windows...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Num < sorted[j].Num })

	for i, w := range sorted {
		shell := children[i]
		pwd, err := fs.Readlink("/proc/" + strconv.Itoa(shell.pid) + "/cwd")
		if err != nil {
			pwd = unknown
		}
		out[w.Num] = Detail{Pwd: pwd, Proc: foreground(procs, shell)}
	}
	return out
}

// foreground remonte au descendant le plus récemment démarré du shell.
func foreground(procs []proc, shell proc) string {
	cur := shell
	for {
		var kids []proc
		for _, p := range procs {
			if p.ppid == cur.pid {
				kids = append(kids, p)
			}
		}
		if len(kids) == 0 {
			return cur.comm
		}
		sort.Slice(kids, func(i, j int) bool { return kids[i].starttime < kids[j].starttime })
		cur = kids[len(kids)-1]
	}
}
