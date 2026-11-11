// Package auth guards the API with a single bearer token stored on disk.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Subprotocol is the WebSocket subprotocol the server negotiates.
const Subprotocol = "betterscreen"

// tokenPrefix marks the token among the offered WebSocket subprotocols:
// browsers cannot set an Authorization header on a WebSocket.
const tokenPrefix = "token."

const minTokenLen = 32

// LoadOrCreate returns the token stored in <dir>/token, generating it (32
// random bytes, base64url) on first use. created reports a fresh token.
func LoadOrCreate(dir string) (token string, created bool, err error) {
	path := filepath.Join(dir, "token")
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		token = strings.TrimSpace(string(data))
		if len(token) < minTokenLen {
			return "", false, fmt.Errorf("token in %s is shorter than %d characters", path, minTokenLen)
		}
		return token, false, nil
	case !errors.Is(err, fs.ErrNotExist):
		return "", false, err
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", false, err
	}
	token = base64.RawURLEncoding.EncodeToString(buf)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", false, err
	}
	if err := os.WriteFile(path, []byte(token+"\n"), 0o600); err != nil {
		return "", false, err
	}
	return token, true, nil
}

// Check reports whether r carries token, either as `Authorization: Bearer`
// or as a `token.<token>` WebSocket subprotocol. An empty token never matches.
func Check(r *http.Request, token string) bool {
	if token == "" {
		return false
	}
	if v, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok && equal(v, token) {
		return true
	}
	for _, line := range r.Header.Values("Sec-WebSocket-Protocol") {
		for _, p := range strings.Split(line, ",") {
			if v, ok := strings.CutPrefix(strings.TrimSpace(p), tokenPrefix); ok && equal(v, token) {
				return true
			}
		}
	}
	return false
}

func equal(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// Middleware rejects requests that fail Check with a JSON 401.
func Middleware(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !Check(r, token) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}` + "\n"))
			return
		}
		next.ServeHTTP(w, r)
	})
}
