package screen

import (
	"regexp"
	"strconv"
)

// Capture: un numéro, des flags optionnels non-espaces, puis le titre.
// Ex: "0 zsh", "1*$ vim". Les titres multi-mots ne sont pas supportés (titres
// screen par défaut = un seul token).
var windowRe = regexp.MustCompile(`(\d+)\S*\s+(\S+)`)

// ParseWindows transforme la sortie de `screen -Q windows` en fenêtres.
func ParseWindows(raw string) []Window {
	var out []Window
	for _, m := range windowRe.FindAllStringSubmatch(raw, -1) {
		num, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		out = append(out, Window{Num: num, Title: m[2]})
	}
	return out
}
