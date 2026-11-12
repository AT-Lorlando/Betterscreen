// Package auth guards the API with a single bearer token stored on disk.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"
)

// Subprotocol is the WebSocket subprotocol the server negotiates.
const Subprotocol = "betterscreen"

// tokenPrefix marks the token among the offered WebSocket subprotocols:
// browsers cannot set an Authorization header on a WebSocket.
const tokenPrefix = "token."

// MinTokenLen is the shortest token the service accepts.
const MinTokenLen = 32

// NewToken returns 32 random bytes encoded as base64url (43 characters).
func NewToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
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
