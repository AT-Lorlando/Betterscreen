// Package serve runs `betterscreen serve`: the HTTPS API service.
package serve

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"betterscreen/internal/api"
	"betterscreen/internal/auth"
	"betterscreen/internal/screen"
	"betterscreen/internal/tlscert"
)

type config struct {
	Addr           string
	ConfigDir      string
	AllowedOrigins []string
}

// originList is a repeatable --allow-origin flag. Each value must be an exact
// browser origin: http(s) scheme, a host, nothing after it.
type originList []string

func (o *originList) String() string { return strings.Join(*o, ",") }

func (o *originList) Set(v string) error {
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
		u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return fmt.Errorf("origin %q must look like https://host[:port]", v)
	}
	*o = append(*o, v)
	return nil
}

func parseFlags(args []string, getenv func(string) string, home string) (config, error) {
	dir := filepath.Join(home, ".config", "betterscreen")
	if x := getenv("XDG_CONFIG_HOME"); x != "" {
		dir = filepath.Join(x, "betterscreen")
	}
	cfg := config{}
	var origins originList
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.StringVar(&cfg.Addr, "addr", ":8443", "listen address")
	fs.StringVar(&cfg.ConfigDir, "config-dir", dir, "directory holding the token and TLS material")
	fs.Var(&origins, "allow-origin", "extra origin allowed by CORS and WebSocket (repeatable)")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	cfg.AllowedOrigins = origins
	return cfg, nil
}

// Run serves the API until ctx is cancelled, then shuts down gracefully.
func Run(ctx context.Context, args []string, stdout io.Writer) error {
	home, _ := os.UserHomeDir()
	cfg, err := parseFlags(args, os.Getenv, home)
	if err != nil {
		return err
	}
	token, created, err := auth.LoadOrCreate(cfg.ConfigDir)
	if err != nil {
		return err
	}
	if created {
		fmt.Fprintf(stdout, "token: %s (saved to %s)\n", token, filepath.Join(cfg.ConfigDir, "token"))
	}
	hosts, ips := tlscert.LocalSANs()
	cert, fingerprint, err := tlscert.LoadOrCreate(cfg.ConfigDir, hosts, ips)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "TLS certificate SHA-256: %s\n", fingerprint)

	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Handler: api.New(api.Options{
			Screen:         screen.NewClient(),
			Token:          token,
			AllowedOrigins: cfg.AllowedOrigins,
			Logger:         log.New(stdout, "", log.LstdFlags),
		}),
		TLSConfig:         &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12},
		ReadHeaderTimeout: 10 * time.Second,
	}
	fmt.Fprintf(stdout, "listening on https://%s\n", ln.Addr())

	errc := make(chan error, 1)
	go func() { errc <- srv.ServeTLS(ln, "", "") }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
