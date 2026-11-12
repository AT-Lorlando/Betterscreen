package serve

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

const goodToken = "0123456789abcdef0123456789abcdef0123456789a"

func TestLoadConfigCreates(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bs")
	fc, created, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !created || len(fc.Token) != 43 {
		t.Fatalf("created = %v, token len %d; want true, 43", created, len(fc.Token))
	}
	for path, want := range map[string]os.FileMode{dir: 0o700, filepath.Join(dir, "config.json"): 0o600} {
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != want {
			t.Errorf("%s mode = %v, want %v", path, fi.Mode().Perm(), want)
		}
	}

	again, created, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if created || again.Token != fc.Token {
		t.Errorf("second load: created = %v, token changed = %v", created, again.Token != fc.Token)
	}
}

func TestLoadConfigReadsFields(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `{"token":"`+goodToken+`","addr":"127.0.0.1:9000","allowed_origins":["https://front.lan"]}`)
	fc, created, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if created || fc.Token != goodToken || fc.Addr != "127.0.0.1:9000" ||
		!slices.Equal(fc.AllowedOrigins, []string{"https://front.lan"}) {
		t.Errorf("got %+v, created %v", fc, created)
	}
}

// A hand-written config without a token gets one, and keeps its other fields.
func TestLoadConfigFillsMissingToken(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `{"addr":":9443"}`)
	fc, created, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !created || len(fc.Token) != 43 || fc.Addr != ":9443" {
		t.Fatalf("got %+v, created %v", fc, created)
	}
	again, _, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if again.Token != fc.Token || again.Addr != ":9443" {
		t.Errorf("token or addr not persisted: %+v", again)
	}
}

func TestLoadConfigRejects(t *testing.T) {
	for name, body := range map[string]string{
		"short token":   `{"token":"abc"}`,
		"unknown field": `{"token":"` + goodToken + `","adress":":1"}`,
		"bad origin":    `{"token":"` + goodToken + `","allowed_origins":["front.lan"]}`,
		"bad json":      `{"token":`,
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeConfig(t, dir, body)
			_, _, err := loadConfig(dir)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), "config.json") {
				t.Errorf("error %q does not name the file", err)
			}
		})
	}
}

func TestMergeConfig(t *testing.T) {
	fc := fileConfig{Token: goodToken, Addr: ":9000", AllowedOrigins: []string{"https://a.lan"}}
	got := merge(config{AllowedOrigins: []string{"https://b.lan"}}, fc)
	if got.Addr != ":9000" {
		t.Errorf("Addr = %q, want config value :9000", got.Addr)
	}
	if want := []string{"https://a.lan", "https://b.lan"}; !slices.Equal(got.AllowedOrigins, want) {
		t.Errorf("AllowedOrigins = %v, want %v", got.AllowedOrigins, want)
	}
	if got := merge(config{Addr: ":1"}, fc); got.Addr != ":1" {
		t.Errorf("flag Addr = %q, want :1 to override the file", got.Addr)
	}
	if got := merge(config{}, fileConfig{}); got.Addr != ":8443" {
		t.Errorf("default Addr = %q, want :8443", got.Addr)
	}
}
