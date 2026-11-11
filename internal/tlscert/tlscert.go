// Package tlscert generates and loads the service's self-signed certificate.
package tlscert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalSANs lists the names this machine is reachable by: localhost, the
// hostname, <hostname>.local, and every non-loopback interface address.
func LocalSANs() (hosts []string, ips []net.IP) {
	hosts = []string{"localhost"}
	if h, err := os.Hostname(); err == nil && h != "" {
		hosts = append(hosts, h, h+".local")
	}
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			ips = append(ips, ipnet.IP)
		}
	}
	return hosts, ips
}

// LoadOrCreate loads <dir>/cert.pem and <dir>/key.pem, generating a fresh
// ECDSA P-256 self-signed pair valid one year when either file is missing.
// fingerprint is the SHA-256 of the certificate, as AB:CD:… hex.
func LoadOrCreate(dir string, hosts []string, ips []net.IP) (cert tls.Certificate, fingerprint string, err error) {
	certPath, keyPath := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	if !exists(certPath) || !exists(keyPath) {
		if err := generate(dir, certPath, keyPath, hosts, ips); err != nil {
			return tls.Certificate{}, "", err
		}
	}
	cert, err = tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return tls.Certificate{}, "", err
	}
	return cert, formatFingerprint(cert.Certificate[0]), nil
}

// formatFingerprint formats the SHA-256 of a DER certificate as AB:CD:… hex.
func formatFingerprint(der []byte) string {
	sum := sha256.Sum256(der)
	parts := make([]string, len(sum))
	for i, b := range sum {
		parts[i] = fmt.Sprintf("%02X", b)
	}
	return strings.Join(parts, ":")
}

func generate(dir, certPath, keyPath string, hosts []string, ips []net.IP) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "betterscreen"},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.AddDate(0, 0, 365),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     hosts,
		IPAddresses:  ips,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := writePEM(keyPath, "PRIVATE KEY", keyDER, 0o600); err != nil {
		return err
	}
	return writePEM(certPath, "CERTIFICATE", der, 0o644)
}

func writePEM(path, typ string, der []byte, mode os.FileMode) error {
	data := pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der})
	if err := os.WriteFile(path, data, mode); err != nil {
		return err
	}
	return os.Chmod(path, mode) // WriteFile keeps the mode of an existing file
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
