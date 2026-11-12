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
	"betterscreen/internal/screen"
	"betterscreen/internal/tlscert"
)

// idleTimeout closes keep-alive connections left idle, so clients without the
// token cannot pile them up. A variable so tests can shorten it.
var idleTimeout = 60 * time.Second

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
	if err := checkOrigin(v); err != nil {
		return err
	}
	*o = append(*o, v)
	return nil
}

func checkOrigin(v string) error {
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
		u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		return fmt.Errorf("origin %q must look like https://host[:port]", v)
	}
	return nil
}

// parseFlags reads the command line. An empty Addr means "not given": merge
// then takes it from config.json, else the default.
func parseFlags(args []string, home string) (config, error) {
	cfg := config{}
	var origins originList
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.StringVar(&cfg.Addr, "addr", "", "listen address (default from config.json, else "+defaultAddr+")")
	fs.StringVar(&cfg.ConfigDir, "config-dir", filepath.Join(home, ".betterscreen"), "directory holding config.json and the TLS material")
	fs.Var(&origins, "allow-origin", "extra origin allowed by CORS and WebSocket (repeatable)")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	cfg.AllowedOrigins = origins
	return cfg, nil
}

func Run(ctx context.Context, args []string, stdout io.Writer) error {
	home, _ := os.UserHomeDir()
	flags, err := parseFlags(args, home)
	if err != nil {
		return err
	}
	fc, created, err := loadConfig(flags.ConfigDir)
	if err != nil {
		return err
	}
	if created {
		fmt.Fprintf(stdout, "token: %s (saved to %s)\n", fc.Token, filepath.Join(flags.ConfigDir, configFile))
	}
	cfg := merge(flags, fc)
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
			Token:          fc.Token,
			AllowedOrigins: cfg.AllowedOrigins,
			Logger:         log.New(stdout, "", log.LstdFlags),
		}),
		TLSConfig:         &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12},
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       idleTimeout,
		// Requests inherit ctx: on shutdown, SSE streams and terminal bridges
		// see it cancelled and unwind instead of holding Shutdown open.
		BaseContext: func(net.Listener) context.Context { return ctx },
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
