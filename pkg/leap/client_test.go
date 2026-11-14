package leap

import (
	"bufio"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestSelectBestIP(t *testing.T) {
	tests := []struct {
		name string
		ipv4 []net.IP
		ipv6 []net.IP
		want string
	}{
		{
			name: "prefer IPv4 over IPv6",
			ipv4: []net.IP{net.ParseIP("192.168.1.50")},
			ipv6: []net.IP{net.ParseIP("2001:db8::1")},
			want: "192.168.1.50",
		},
		{
			name: "reject link-local IPv6 when no IPv4",
			ipv4: nil,
			ipv6: []net.IP{net.ParseIP("fe80::1")},
			want: "",
		},
		{
			name: "accept global IPv6 when no IPv4",
			ipv4: nil,
			ipv6: []net.IP{net.ParseIP("2001:db8::1")},
			want: "2001:db8::1",
		},
		{
			name: "reject loopback IPv4 and IPv6",
			ipv4: []net.IP{net.ParseIP("127.0.0.1")},
			ipv6: []net.IP{net.ParseIP("::1")},
			want: "",
		},
		{
			name: "empty inputs return empty string",
			ipv4: nil,
			ipv6: nil,
			want: "",
		},
		{
			name: "fallback to second IPv4 if first is loopback",
			ipv4: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("192.168.1.51")},
			ipv6: nil,
			want: "192.168.1.51",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectBestIP(tt.ipv4, tt.ipv6)
			if got != tt.want {
				t.Errorf("got: %q, want: %q", got, tt.want)
			}
		})
	}
}

func TestSetLogger(t *testing.T) {
	var buf bytes.Buffer
	customLogger := log.New(&buf, "TEST: ", 0)

	SetLogger(customLogger)
	t.Cleanup(func() {
		SetLogger(nil)
	})

	l := getLogger()
	if l != customLogger {
		t.Errorf("leap.getLogger() = %v, want %v", l, customLogger)
	}

	l.Print("hello world")
	got := buf.String()
	want := "TEST: hello world\n"
	if got != want {
		t.Errorf("got: %q, want: %q", got, want)
	}
}

func TestVerifyBridgePeerCert(t *testing.T) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey(CA) failed: %v", err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Lutron Test Root CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("x509.CreateCertificate(CA) failed: %v", err)
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatalf("x509.ParseCertificate(CA) failed: %v", err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(caCert)

	// Leaf cert with bridge CN (no IP SAN) signed by trusted CA
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey(leaf) failed: %v", err)
	}
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "lutron-01234567.local"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	validLeafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("x509.CreateCertificate(validLeaf) failed: %v", err)
	}

	// Self-signed rogue cert not rooted in pool
	rogueKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey(rogue) failed: %v", err)
	}
	rogueDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, leafTmpl, &rogueKey.PublicKey, rogueKey)
	if err != nil {
		t.Fatalf("x509.CreateCertificate(rogue) failed: %v", err)
	}

	verifier := verifyBridgePeerCert(pool)

	if err := verifier([][]byte{validLeafDER}, nil); err != nil {
		t.Errorf("verifyBridgePeerCert(validLeaf) = %v, want nil", err)
	}
	if err := verifier([][]byte{rogueDER}, nil); err == nil {
		t.Errorf("verifyBridgePeerCert(rogueLeaf) = nil, want CA verification error")
	}
	if err := verifier(nil, nil); err == nil {
		t.Errorf("verifyBridgePeerCert(empty) = nil, want error")
	}
}

func writeTestPKI(t *testing.T, dir string) (caFile, serverCertFile, serverKeyFile, clientCertFile, clientKeyFile string) {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("ecdsa.GenerateKey(CA) failed: %v", err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(10),
		Subject:               pkix.Name{CommonName: "Lutron Mock CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("x509.CreateCertificate(CA) failed: %v", err)
	}
	caCert, _ := x509.ParseCertificate(caDER)
	caFile = filepath.Join(dir, "ca.crt")
	_ = os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), 0600)

	makeLeaf := func(cn string, serial int64, usage x509.ExtKeyUsage, certName, keyName string) (string, string) {
		k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			t.Fatalf("ecdsa.GenerateKey(%s) failed: %v", cn, err)
		}
		tmpl := &x509.Certificate{
			SerialNumber: big.NewInt(serial),
			Subject:      pkix.Name{CommonName: cn},
			NotBefore:    time.Now().Add(-time.Hour),
			NotAfter:     time.Now().Add(time.Hour),
			KeyUsage:     x509.KeyUsageDigitalSignature,
			ExtKeyUsage:  []x509.ExtKeyUsage{usage},
		}
		der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &k.PublicKey, caKey)
		if err != nil {
			t.Fatalf("x509.CreateCertificate(%s) failed: %v", cn, err)
		}
		keyBytes, _ := x509.MarshalECPrivateKey(k)
		cPath := filepath.Join(dir, certName)
		kPath := filepath.Join(dir, keyName)
		_ = os.WriteFile(cPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600)
		_ = os.WriteFile(kPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes}), 0600)
		return cPath, kPath
	}

	serverCertFile, serverKeyFile = makeLeaf("lutron-bridge.local", 11, x509.ExtKeyUsageServerAuth, "server.crt", "server.key")
	clientCertFile, clientKeyFile = makeLeaf("homectl-client", 12, x509.ExtKeyUsageClientAuth, "client.crt", "client.key")
	return
}

func TestClient_LEAPOperationsAndReconnect(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	pkiDir := t.TempDir()
	caFile, srvCertFile, srvKeyFile, cliCertFile, cliKeyFile := writeTestPKI(t, pkiDir)

	srvCert, err := tls.LoadX509KeyPair(srvCertFile, srvKeyFile)
	if err != nil {
		t.Fatalf("tls.LoadX509KeyPair(server) failed: %v", err)
	}
	caPEM, _ := os.ReadFile(caFile)
	clientPool := x509.NewCertPool()
	clientPool.AppendCertsFromPEM(caPEM)

	ln, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{srvCert},
		ClientCAs:    clientPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
	})
	if err != nil {
		t.Fatalf("tls.Listen(127.0.0.1:0) failed: %v", err)
	}
	defer ln.Close()

	var dropNext atomic.Bool
	go func() {
		for {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				sc := bufio.NewScanner(c)
				for sc.Scan() {
					if dropNext.CompareAndSwap(true, false) {
						// Simulate abrupt bridge TCP reset to exercise Request() auto-reconnect
						return
					}
					var req Message
					if json.Unmarshal(sc.Bytes(), &req) != nil {
						continue
					}
					// Emit an unsolicited status event first (no ClientTag) to verify tag correlation
					unsolicited, _ := json.Marshal(Message{
						CommuniqueType: "ReadResponse",
						Header:         Header{Url: "/zone/status"},
					})
					_, _ = c.Write(append(unsolicited, '\n'))

					var body any
					switch req.Header.Url {
					case "/area":
						body = AreaResponse{Areas: []Area{{Href: "/area/1", Name: "Living Room"}}}
					case "/device":
						body = DeviceResponse{Devices: []Device{{Href: "/device/1", Name: "Dimmer", DeviceType: "WallDimmer"}}}
					case "/zone":
						body = ZoneResponse{Zones: []Zone{{Href: "/zone/1", Name: "Main Lights", ControlType: "Dimmed"}}}
					case "/zone/status":
						body = ZoneStatusResponse{ZoneStatuses: []ZoneStatus{{Href: "/zone/1/status", Level: 55}}}
					case "/zone/1/status":
						zs := ZoneStatus{Href: "/zone/1/status", Level: 55}
						body = ZoneStatusResponse{ZoneStatus: &zs}
					default:
						body = map[string]any{}
					}
					rawBody, _ := json.Marshal(body)
					respBytes, _ := json.Marshal(Message{
						CommuniqueType: "ReadResponse",
						Header:         Header{ClientTag: req.Header.ClientTag, Url: req.Header.Url},
						Body:           rawBody,
					})
					_, _ = c.Write(append(respBytes, '\n'))
				}
			}(conn)
		}
	}()

	client, err := NewClient(ln.Addr().String(), cliCertFile, cliKeyFile, caFile)
	if err != nil {
		t.Fatalf("NewClient(%q) failed: %v", ln.Addr().String(), err)
	}
	defer client.Close()

	if err := client.ConnectContext(t.Context()); err != nil {
		t.Fatalf("client.ConnectContext(ctx) failed: %v", err)
	}

	areas, err := client.GetAreas()
	if err != nil || len(areas) != 1 || areas[0].Name != "Living Room" {
		t.Errorf("client.GetAreas(/area) = (%+v, %v), want 1 area Living Room", areas, err)
	}

	devices, err := client.GetDevices()
	if err != nil || len(devices) != 1 || devices[0].Name != "Dimmer" {
		t.Errorf("client.GetDevices(/device) = (%+v, %v), want 1 device Dimmer", devices, err)
	}

	// Trigger EOF drop on next request to verify automatic reconnect + retry
	dropNext.Store(true)
	zones, err := client.GetZones()
	if err != nil || len(zones) != 1 || zones[0].Href != "/zone/1" {
		t.Errorf("client.GetZones(/zone after reconnect) = (%+v, %v), want /zone/1", zones, err)
	}

	statuses, err := client.GetAllZoneStatuses()
	if err != nil || len(statuses) != 1 || statuses[0].Level != 55 {
		t.Errorf("client.GetAllZoneStatuses(/zone/status) = (%+v, %v), want Level=55", statuses, err)
	}

	single, err := client.GetZoneStatus("/zone/1")
	if err != nil || single.Level != 55 {
		t.Errorf("client.GetZoneStatus(/zone/1) = (%+v, %v), want Level=55", single, err)
	}

	if err := client.SetLevel("/zone/1", 75); err != nil {
		t.Errorf("client.SetLevel(/zone/1, 75) = %v, want nil", err)
	}
	if err := client.SetAllLevels(0); err != nil {
		t.Errorf("client.SetAllLevels(0) = %v, want nil", err)
	}

	// Cache round-trip
	if err := SaveCache([]Bridge{{Name: "Smart Bridge", IP: "192.168.1.20"}}); err != nil {
		t.Fatalf("SaveCache(bridges) failed: %v", err)
	}
	cached, err := LoadCache()
	if err != nil || len(cached) != 1 || cached[0].IP != "192.168.1.20" {
		t.Errorf("LoadCache(bridges) = (%+v, %v), want 192.168.1.20", cached, err)
	}
}
