package roborock

import (
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// CertPair references a PEM fullchain certificate and private key on disk.
type CertPair struct {
	CertFile string `json:"cert_file"`
	KeyFile  string `json:"key_file"`
}

type cachedCert struct {
	pair        CertPair
	cert        *tls.Certificate
	certModTime time.Time
	keyModTime  time.Time
	lastChecked time.Time
}

// MultiSNICertManager serves per-SNI TLS certificates with automatic mtime-based hot-reload.
// This allows serving e.g. a Porkbun/ZeroSSL/Let's Encrypt wildcard cert for Saros 10R/20
// and an Actalis RSA-2048 cert for older vacuums from a single HTTPS/MQTT listener.
type MultiSNICertManager struct {
	mu            sync.RWMutex
	entries       map[string]*cachedCert // lowercase exact or "*.domain" or "*" default
	defaultKey    string
	checkInterval time.Duration
}

// NewMultiSNICertManager loads and validates one or more SNI-mapped certificate pairs.
// Keys may be exact hostnames ("api-r.example.com"), wildcards ("*.example.com"), or "*" / "" for default.
func NewMultiSNICertManager(certs map[string]CertPair) (*MultiSNICertManager, error) {
	if len(certs) == 0 {
		return nil, errors.New("roborock: at least one TLS certificate pair is required")
	}
	m := &MultiSNICertManager{
		entries:       make(map[string]*cachedCert, len(certs)),
		checkInterval: 5 * time.Second,
	}
	for host, pair := range certs {
		key := strings.ToLower(strings.TrimSpace(host))
		if key == "" {
			key = "*"
		}
		entry := &cachedCert{pair: pair}
		if err := m.loadEntryLocked(entry); err != nil {
			return nil, fmt.Errorf("roborock: load TLS cert for %q: %w", key, err)
		}
		m.entries[key] = entry
		if m.defaultKey == "" || key == "*" {
			m.defaultKey = key
		}
	}
	return m, nil
}

// NewMemoryCertManager builds a MultiSNICertManager directly from an in-memory tls.Certificate (for tests).
func NewMemoryCertManager(cert tls.Certificate) *MultiSNICertManager {
	c := cert
	return &MultiSNICertManager{
		entries: map[string]*cachedCert{
			"*": {cert: &c},
		},
		defaultKey:    "*",
		checkInterval: time.Hour,
	}
}

func (m *MultiSNICertManager) loadEntryLocked(entry *cachedCert) error {
	if entry.pair.CertFile == "" || entry.pair.KeyFile == "" {
		if entry.cert != nil {
			return nil
		}
		return errors.New("cert_file and key_file are required")
	}
	cInfo, err := os.Stat(entry.pair.CertFile)
	if err != nil {
		return err
	}
	kInfo, err := os.Stat(entry.pair.KeyFile)
	if err != nil {
		return err
	}
	if entry.cert != nil && cInfo.ModTime().Equal(entry.certModTime) && kInfo.ModTime().Equal(entry.keyModTime) {
		entry.lastChecked = time.Now()
		return nil
	}
	loaded, err := tls.LoadX509KeyPair(entry.pair.CertFile, entry.pair.KeyFile)
	if err != nil {
		return err
	}
	entry.cert = &loaded
	entry.certModTime = cInfo.ModTime()
	entry.keyModTime = kInfo.ModTime()
	entry.lastChecked = time.Now()
	return nil
}

func (m *MultiSNICertManager) lookupKeyLocked(serverName string) string {
	host := strings.ToLower(strings.TrimSpace(serverName))
	if host != "" {
		if _, ok := m.entries[host]; ok {
			return host
		}
		if idx := strings.IndexByte(host, '.'); idx >= 0 && idx+1 < len(host) {
			wild := "*" + host[idx:]
			if _, ok := m.entries[wild]; ok {
				return wild
			}
		}
	}
	if _, ok := m.entries["*"]; ok {
		return "*"
	}
	return m.defaultKey
}

// GetCertificate implements tls.Config.GetCertificate with automatic disk hot-reload.
func (m *MultiSNICertManager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	serverName := ""
	if hello != nil {
		serverName = hello.ServerName
	}

	m.mu.RLock()
	key := m.lookupKeyLocked(serverName)
	entry := m.entries[key]
	if entry == nil {
		m.mu.RUnlock()
		return nil, fmt.Errorf("roborock: no TLS certificate available for %q", serverName)
	}
	needsCheck := entry.pair.CertFile != "" && time.Since(entry.lastChecked) >= m.checkInterval
	cert := entry.cert
	m.mu.RUnlock()

	if !needsCheck {
		return cert, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.loadEntryLocked(entry); err != nil {
		// Keep serving last-known-good certificate if a transient write/rename is in progress.
		if entry.cert != nil {
			return entry.cert, nil
		}
		return nil, err
	}
	return entry.cert, nil
}

// TLSConfig constructs a *tls.Config using GetCertificate.
// When allowLegacy is true, MinVersion is lowered to TLS 1.0 for older Roborock firmware stacks.
func (m *MultiSNICertManager) TLSConfig(allowLegacy bool) *tls.Config {
	minVer := uint16(tls.VersionTLS12)
	if allowLegacy {
		minVer = tls.VersionTLS10
	}
	return &tls.Config{
		MinVersion:     minVer,
		GetCertificate: m.GetCertificate,
	}
}
