package serve

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"betterscreen/internal/auth"
)

const (
	configFile  = "config.json"
	defaultAddr = ":8443"
)

// fileConfig is the content of <config-dir>/config.json.
type fileConfig struct {
	Token          string   `json:"token"`
	Addr           string   `json:"addr,omitempty"`
	AllowedOrigins []string `json:"allowed_origins,omitempty"`
}

// loadConfig reads <dir>/config.json. A missing file, or one without a token,
// gets a fresh token written back (dir 0700, file 0600); created reports it.
func loadConfig(dir string) (fc fileConfig, created bool, err error) {
	path := filepath.Join(dir, configFile)
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&fc); err != nil {
			return fileConfig{}, false, fmt.Errorf("%s: %w", path, err)
		}
	case !errors.Is(err, fs.ErrNotExist):
		return fileConfig{}, false, err
	}
	for _, o := range fc.AllowedOrigins {
		if err := checkOrigin(o); err != nil {
			return fileConfig{}, false, fmt.Errorf("%s: %w", path, err)
		}
	}
	if fc.Token != "" {
		if len(fc.Token) < auth.MinTokenLen {
			return fileConfig{}, false, fmt.Errorf("%s: token is shorter than %d characters", path, auth.MinTokenLen)
		}
		return fc, false, nil
	}

	if fc.Token, err = auth.NewToken(); err != nil {
		return fileConfig{}, false, err
	}
	out, err := json.MarshalIndent(fc, "", "  ")
	if err != nil {
		return fileConfig{}, false, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fileConfig{}, false, err
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o600); err != nil {
		return fileConfig{}, false, err
	}
	return fc, true, nil
}

// merge applies the precedence flag > config.json > default. Origins add up.
func merge(flags config, fc fileConfig) config {
	cfg := flags
	if cfg.Addr == "" {
		cfg.Addr = fc.Addr
	}
	if cfg.Addr == "" {
		cfg.Addr = defaultAddr
	}
	cfg.AllowedOrigins = append(append([]string{}, fc.AllowedOrigins...), flags.AllowedOrigins...)
	return cfg
}
