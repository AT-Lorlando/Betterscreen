package tlscert

import (
	"crypto/x509"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
)

var fpRe = regexp.MustCompile(`^([0-9A-F]{2}:){31}[0-9A-F]{2}$`)

func TestLoadOrCreateGenerates(t *testing.T) {
	dir := t.TempDir()
	hosts := []string{"localhost", "box", "box.local"}
	ips := []net.IP{net.ParseIP("192.168.1.20")}
	cert, fp, err := LoadOrCreate(dir, hosts, ips)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(leaf.DNSNames, hosts) {
		t.Errorf("DNSNames = %v, want %v", leaf.DNSNames, hosts)
	}
	if len(leaf.IPAddresses) != 1 || !leaf.IPAddresses[0].Equal(ips[0]) {
		t.Errorf("IPAddresses = %v, want %v", leaf.IPAddresses, ips)
	}
	for name, want := range map[string]os.FileMode{"key.pem": 0o600, "cert.pem": 0o644} {
		fi, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if fi.Mode().Perm() != want {
			t.Errorf("%s mode = %v, want %v", name, fi.Mode().Perm(), want)
		}
	}
	if !fpRe.MatchString(fp) {
		t.Errorf("fingerprint %q has the wrong format", fp)
	}
}

func TestLoadOrCreateReloads(t *testing.T) {
	dir := t.TempDir()
	_, first, err := LoadOrCreate(dir, []string{"localhost"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, second, err := LoadOrCreate(dir, []string{"localhost"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Errorf("fingerprint changed on reload: %s then %s", first, second)
	}
}

func TestLoadOrCreateRegeneratesWhenMissing(t *testing.T) {
	dir := t.TempDir()
	_, first, err := LoadOrCreate(dir, []string{"localhost"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "cert.pem")); err != nil {
		t.Fatal(err)
	}
	_, second, err := LoadOrCreate(dir, []string{"localhost"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Error("fingerprint unchanged after cert.pem was deleted")
	}
}
