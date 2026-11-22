package roborock

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ghchinoy/homectl/modules/core"
)

func generateSelfSignedCertFiles(t *testing.T, dir, name string, dnsNames []string) CertPair {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: dnsNames[0]},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     dnsNames,
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	certPath := filepath.Join(dir, name+".crt")
	keyPath := filepath.Join(dir, name+".key")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0600); err != nil {
		t.Fatal(err)
	}
	return CertPair{CertFile: certPath, KeyFile: keyPath}
}

func TestMultiSNICertManagerAndHotReload(t *testing.T) {
	dir := t.TempDir()
	pair1 := generateSelfSignedCertFiles(t, dir, "saros", []string{"*.example.com", "example.com"})
	pair2 := generateSelfSignedCertFiles(t, dir, "legacy", []string{"api-legacy.example.com"})

	mgr, err := NewMultiSNICertManager(map[string]CertPair{
		"*.example.com":          pair1,
		"api-legacy.example.com": pair2,
	})
	if err != nil {
		t.Fatalf("NewMultiSNICertManager: %v", err)
	}
	mgr.checkInterval = time.Millisecond

	c1, err := mgr.GetCertificate(&tls.ClientHelloInfo{ServerName: "api-r.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	leaf1, _ := x509.ParseCertificate(c1.Certificate[0])
	if leaf1.Subject.CommonName != "*.example.com" {
		t.Fatalf("wildcard SNI CN: got: %q, want: %q", leaf1.Subject.CommonName, "*.example.com")
	}

	c2, err := mgr.GetCertificate(&tls.ClientHelloInfo{ServerName: "api-legacy.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	leaf2, _ := x509.ParseCertificate(c2.Certificate[0])
	if leaf2.Subject.CommonName != "api-legacy.example.com" {
		t.Fatalf("exact SNI CN: got: %q, want: %q", leaf2.Subject.CommonName, "api-legacy.example.com")
	}

	// Simulate certificate renewal on disk and verify hot-reload picks up new serial.
	oldSerial := leaf2.SerialNumber
	time.Sleep(15 * time.Millisecond)
	_ = generateSelfSignedCertFiles(t, dir, "legacy", []string{"api-legacy.example.com"})
	future := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(pair2.CertFile, future, future)
	_ = os.Chtimes(pair2.KeyFile, future, future)

	c2New, err := mgr.GetCertificate(&tls.ClientHelloInfo{ServerName: "api-legacy.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	leaf2New, _ := x509.ParseCertificate(c2New.Certificate[0])
	if leaf2New.SerialNumber.Cmp(oldSerial) == 0 {
		t.Fatalf("hot-reload did not refresh certificate serial")
	}
}

func TestCameraBootstrapValidation(t *testing.T) {
	// 14 chars fits <= 15 for both Saros 10R (country_domain) and Saros 20 (token.r) on port 443.
	eps10R, err := CameraBootstrapEndpoints("roborock.vacuum.a144", "r.example.net/", "r.example.net/")
	if err != nil || len(eps10R) != 1 || eps10R[0].Port != 443 {
		t.Fatalf("Saros 10R camera endpoints: got: %+v err=%v", eps10R, err)
	}
	eps20, err := CameraBootstrapEndpoints("roborock.vacuum.a288", "r.example.net/", "us")
	if err != nil || len(eps20) != 1 || eps20[0].InputName != "token.r" {
		t.Fatalf("Saros 20 camera endpoints: got: %+v err=%v", eps20, err)
	}
	// 16 chars (>15) must fail validation.
	if _, err := CameraBootstrapEndpoints("roborock.vacuum.a288", "example.com:555/", "us"); err == nil {
		t.Fatalf("expected error for 16-char token.r")
	}
}

type mockMultiKeyRecoverer struct {
	keysByHash map[string]*rsa.PublicKey
}

func (m *mockMultiKeyRecoverer) RecoverModulus(_ context.Context, _ []SignatureSample, hashName string) (*RecoveryResult, error) {
	pub := m.keysByHash[hashName]
	if pub == nil {
		return nil, fmt.Errorf("no stub key for %s", hashName)
	}
	return &RecoveryResult{Modulus: pub.N, Duration: time.Millisecond, Backend: "mock"}, nil
}

// TestEndToEndMockV1AndV2Fleet spins up the full Stack (HTTPS + Embedded Mochi MQTT + Embedded Pion TURN)
// and simulates a V1 Q5 Max+ (a125), a V2 Saros 10R (a144), and a V2 Saros 20 (a288).
func TestEndToEndMockV1AndV2Fleet(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	v1Priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	v2Priv, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		t.Fatal(err)
	}

	certDir := t.TempDir()
	pair := generateSelfSignedCertFiles(t, certDir, "stack", []string{"api-r.example.com", "r.example.com", "localhost"})
	certMgr, err := NewMultiSNICertManager(map[string]CertPair{"*": pair})
	if err != nil {
		t.Fatal(err)
	}

	// Pick free ports for HTTPS, MQTT, and TURN.
	httpsPort := pickFreeTCPPort(t)
	mqttPort := pickFreeTCPPort(t)
	turnPort := pickFreeUDPPort(t)

	stack, err := NewStack(StackConfig{
		Storage: core.NewDirStorage(t.TempDir()),
		Logger:  core.NewNoOpLogger(),
		Recoverer: &mockMultiKeyRecoverer{keysByHash: map[string]*rsa.PublicKey{
			"sha256": &v1Priv.PublicKey,
			"sha384": &v2Priv.PublicKey,
		}},
		Certs: certMgr,
		HTTP: HTTPServerConfig{
			BindHost:           "127.0.0.1",
			Port:               httpsPort,
			AdvertisedHost:     "api-r.example.com",
			AdvertisedPort:     443,
			AdvertisedMQTT:     "api-r.example.com",
			AdvertisedMQTTPort: mqttPort,
			Region:             "us",
			Timezone:           "America/Denver",
		},
		MQTT: MQTTBrokerConfig{
			BindHost:              "127.0.0.1",
			Port:                  mqttPort,
			DisableTLS:            true, // keep mock raw MQTT client simple in unit test
			NewConnectionsEnabled: true,
		},
		TURN: TURNConfig{
			Mode:     TURNModeEmbedded,
			BindHost: "127.0.0.1",
			Host:     "r.example.com",
			PublicIP: "127.0.0.1",
			Port:     turnPort,
			Username: "rrturn",
			Password: "test-secret-password",
			Realm:    "r.example.com",
		},
		NewConnectionsEnabled: true,
	})
	if err != nil {
		t.Fatalf("NewStack: %v", err)
	}
	if err := stack.Start(ctx); err != nil {
		t.Fatalf("stack.Start: %v", err)
	}
	defer func() { _ = stack.Close(context.Background()) }()

	if !stack.TURN.Running() {
		t.Fatalf("embedded Pion TURN relay should be running")
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Timeout: 5 * time.Second,
	}
	baseURL := fmt.Sprintf("https://127.0.0.1:%d", httpsPort)

	// --- 1. Simulate V1 Q5 Max+ (a125) Onboarding: 2x query-signed GET /.roborock.com/region ---
	for i := 1; i <= 2; i++ {
		q := fmt.Sprintf("did=125001&pid=roborock.vacuum.a125&ts=%d&nonce=n%d", 1700000000+i, i)
		h := sha256.Sum256([]byte(q))
		sig, err := rsa.SignPKCS1v15(rand.Reader, v1Priv, crypto.SHA256, h[:])
		if err != nil {
			t.Fatal(err)
		}
		resp, err := httpClient.Get(baseURL + "/.roborock.com/region?" + q + "&signature=" + url.QueryEscape(base64.StdEncoding.EncodeToString(sig)))
		if err != nil {
			t.Fatalf("V1 region #%d: %v", i, err)
		}
		_ = resp.Body.Close()
	}
	stack.Keys.WaitRecoveries()
	if stack.Keys.GetPublicKey("125001") == nil {
		t.Fatalf("V1 a125 public key was not recovered")
	}

	// 3rd /region call must return OAEP-encrypted string in "result"
	resp, err := httpClient.Get(baseURL + "/.roborock.com/region?did=125001&pid=roborock.vacuum.a125")
	if err != nil {
		t.Fatal(err)
	}
	var regResp struct {
		Code   int    `json:"code"`
		Result string `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&regResp); err != nil {
		t.Fatalf("decode encrypted /region: %v", err)
	}
	_ = resp.Body.Close()
	encBytes, err := base64.StdEncoding.DecodeString(regResp.Result)
	if err != nil {
		t.Fatal(err)
	}
	decRegion, err := rsa.DecryptOAEP(sha1.New(), rand.Reader, v1Priv, encBytes, nil)
	if err != nil {
		t.Fatalf("decrypt /region OAEP: %v", err)
	}
	if !strings.Contains(string(decRegion), `"mqttUrl":"ssl://api-r.example.com:`) {
		t.Fatalf("decrypted /region missing mqttUrl: got: %s", string(decRegion))
	}

	// V1 POST /f/c rrcheck grant forgery
	fcResp, err := httpClient.Post(baseURL+"/f/c?did=125001", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	var grant RRCheckGrant
	if err := json.NewDecoder(fcResp.Body).Decode(&grant); err != nil {
		t.Fatalf("decode /f/c grant: %v", err)
	}
	_ = fcResp.Body.Close()
	if grant.K == "" || grant.D == "" {
		t.Fatalf("/f/c grant empty: got: %+v", grant)
	}

	// --- 2. Simulate V2 Saros 10R (a144) Onboarding: 2x header-signed GET /region + multipart POST /nc ---
	for i := 1; i <= 2; i++ {
		q := fmt.Sprintf("did=144001&pid=roborock.vacuum.a144&seq=%d", i)
		nonce := fmt.Sprintf("v2n%d", i)
		ts := fmt.Sprintf("%d", 1700001000+i)
		canon := fmt.Sprintf("%s:%s:%s", q, nonce, ts)
		h := sha512.Sum384([]byte(canon))
		sig, err := rsa.SignPKCS1v15(rand.Reader, v2Priv, crypto.SHA384, h[:])
		if err != nil {
			t.Fatal(err)
		}
		req, _ := http.NewRequest(http.MethodGet, baseURL+"/.roborock.com/region?"+q, nil)
		req.Header.Set("X-Signature", base64.StdEncoding.EncodeToString(sig))
		req.Header.Set("X-Nonce", nonce)
		req.Header.Set("X-Timestamp", ts)
		req.Header.Set("X-Version", "v2")
		r, err := httpClient.Do(req)
		if err != nil {
			t.Fatalf("V2 region #%d: %v", i, err)
		}
		_ = r.Body.Close()
	}
	stack.Keys.WaitRecoveries()
	if stack.Keys.GetPublicKey("144001") == nil {
		t.Fatalf("V2 a144 public key was not recovered")
	}

	// Multipart POST /nc (V2 Saros 10R/20 contract)
	var mpBuf bytes.Buffer
	mw := multipart.NewWriter(&mpBuf)
	_ = mw.WriteField("did", "144001")
	_ = mw.WriteField("pid", "roborock.vacuum.a144")
	_ = mw.WriteField("s", "sess144")
	_ = mw.WriteField("t", "tok144")
	_ = mw.Close()
	ncReq, _ := http.NewRequest(http.MethodPost, baseURL+"/.roborock.com/nc", &mpBuf)
	ncReq.Header.Set("Content-Type", mw.FormDataContentType())
	ncResp, err := httpClient.Do(ncReq)
	if err != nil {
		t.Fatal(err)
	}
	var ncBody struct {
		Code   int    `json:"code"`
		Result string `json:"result"`
	}
	if err := json.NewDecoder(ncResp.Body).Decode(&ncBody); err != nil {
		t.Fatal(err)
	}
	_ = ncResp.Body.Close()
	ncEnc, _ := base64.StdEncoding.DecodeString(ncBody.Result)
	ncPlain, err := rsa.DecryptOAEP(sha1.New(), rand.Reader, v2Priv, ncEnc, nil)
	if err != nil {
		t.Fatalf("decrypt V2 /nc OAEP: %v", err)
	}
	var ncJSON map[string]string
	if err := json.Unmarshal(ncPlain, &ncJSON); err != nil {
		t.Fatal(err)
	}
	localKey144 := ncJSON["k"]
	if len(localKey144) != 16 {
		t.Fatalf("minted a144 localKey: got: %q (len %d), want 16 chars", localKey144, len(localKey144))
	}

	// V2 GET /devices/144001/sh rrcheck renewal + POST /iot.roborock.com/fwapi/createca TURN
	shResp, err := httpClient.Get(baseURL + "/devices/144001/sh")
	if err != nil {
		t.Fatal(err)
	}
	var shBody map[string]any
	if err := json.NewDecoder(shResp.Body).Decode(&shBody); err != nil {
		t.Fatal(err)
	}
	_ = shResp.Body.Close()
	if shBody["k"] == "" || shBody["d"] == "" {
		t.Fatalf("/devices/144001/sh missing k/d grant: %+v", shBody)
	}

	caResp, err := httpClient.Post(baseURL+"/iot.roborock.com/fwapi/createca", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	var caBody struct {
		Code int         `json:"code"`
		Data TURNPayload `json:"data"`
	}
	if err := json.NewDecoder(caResp.Body).Decode(&caBody); err != nil {
		t.Fatal(err)
	}
	_ = caResp.Body.Close()
	wantTURN := fmt.Sprintf("turn:r.example.com:%d", turnPort)
	if caBody.Code != 200 || caBody.Data.URL != wantTURN || caBody.Data.Username != "rrturn" {
		t.Fatalf("/fwapi/createca mismatch: got: %+v, want url=%s", caBody, wantTURN)
	}

	// --- 3. Simulate Live MQTT Connection, Telemetry Tap, Firmware Report & Active RPC Round-Trip ---
	mqttConn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", mqttPort), 3*time.Second)
	if err != nil {
		t.Fatalf("dial embedded mqtt: %v", err)
	}
	defer mqttConn.Close()

	// Send MQTT 3.1.1 CONNECT with provisional device creds
	sendRawMQTTConnect(t, mqttConn, "vac-144", "vacusr144", "vacpass144")
	// Subscribe to outbound command topic rr/d/o/144001/vacusr144
	sendRawMQTTSubscribe(t, mqttConn, 1, "rr/d/o/144001/vacusr144")

	// Publish encrypted protocol-203 firmware report & DPS battery/state on rr/d/i/144001/vacusr144
	otaWire, err := EncodeMessage(Message{
		Version:   Version10,
		Seq:       10,
		Random:    11,
		Timestamp: uint32(time.Now().Unix()),
		Protocol:  ProtoOTAFirmwareReport,
		Payload:   []byte(`{"mqttOtaData":{"mqttOtaFirmwareVersion":{"version":"02.52.86"}}}`),
	}, CodecOptions{LocalKey: localKey144})
	if err != nil {
		t.Fatal(err)
	}
	sendRawMQTTPublish(t, mqttConn, "rr/d/i/144001/vacusr144", otaWire)

	dpsWire, err := EncodeMessage(Message{
		Version:   Version10,
		Seq:       11,
		Random:    12,
		Timestamp: uint32(time.Now().Unix()),
		Protocol:  ProtoRPCResponse,
		Payload:   []byte(`{"dps":{"122":94,"121":8,"120":0}}`),
	}, CodecOptions{LocalKey: localKey144})
	if err != nil {
		t.Fatal(err)
	}
	sendRawMQTTPublish(t, mqttConn, "rr/d/i/144001/vacusr144", dpsWire)

	// Wait briefly for broker hook to process telemetry
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		snaps := stack.Telemetry.Snapshot(stack.Creds, stack.Keys)
		for _, v := range snaps {
			if v.DID == "144001" && v.Battery != nil && *v.Battery == 94 && v.FirmwareVersion == "02.52.86" {
				goto telemetryVerified
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("telemetry hub did not record battery=94 and fv=02.52.86: %+v", stack.Telemetry.Snapshot(stack.Creds, stack.Keys))

telemetryVerified:
	// Spawn mock vacuum responder goroutine that reads rr/d/o/144001/vacusr144 and replies on rr/d/i/144001/vacusr144
	rpcErrCh := make(chan error, 1)
	go func() {
		topic, payload, err := readRawMQTTPublish(mqttConn)
		if err != nil {
			rpcErrCh <- err
			return
		}
		if topic != "rr/d/o/144001/vacusr144" {
			rpcErrCh <- fmt.Errorf("unexpected outbound topic %q", topic)
			return
		}
		msgs, _, err := DecodeMessages(payload, CodecOptions{LocalKey: localKey144})
		if err != nil || len(msgs) == 0 {
			rpcErrCh <- fmt.Errorf("decode rpc req: %v", err)
			return
		}
		var env map[string]any
		_ = json.Unmarshal(msgs[0].Payload, &env)
		dps, _ := env["dps"].(map[string]any)
		reqObj, _ := extractRPCFromDPS(dps, "101")
		reqID, _ := anyToInt(reqObj["id"])
		replyInner, _ := json.Marshal(map[string]any{
			"id":     reqID,
			"result": []any{map[string]any{"battery": 95, "state": 8, "error_code": 0}},
		})
		replyEnv, _ := json.Marshal(map[string]any{
			"t":   time.Now().Unix(),
			"dps": map[string]any{"102": string(replyInner)},
		})
		replyWire, err := EncodeMessage(Message{
			Version:   Version10,
			Seq:       99,
			Random:    100,
			Timestamp: uint32(time.Now().Unix()),
			Protocol:  ProtoRPCResponse,
			Payload:   replyEnv,
		}, CodecOptions{LocalKey: localKey144})
		if err != nil {
			rpcErrCh <- err
			return
		}
		sendRawMQTTPublish(t, mqttConn, "rr/d/i/144001/vacusr144", replyWire)
		rpcErrCh <- nil
	}()

	rpcCtx, rpcCancel := context.WithTimeout(ctx, 3*time.Second)
	defer rpcCancel()
	rpcRes, err := stack.Controller.GetStatus(rpcCtx, "a144")
	if err != nil {
		t.Fatalf("Controller.GetStatus(a144): %v", err)
	}
	if err := <-rpcErrCh; err != nil {
		t.Fatalf("mock vacuum responder error: %v", err)
	}
	if !strings.Contains(string(rpcRes.Result), `"battery":95`) {
		t.Fatalf("RPC GetStatus result: got: %s, want battery:95", string(rpcRes.Result))
	}
}

func TestLocalTCPClientL01HandshakeAndRPC(t *testing.T) {
	localKey := "0011223344556677"
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	ackNonce := uint32(0x22334455)
	done := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			done <- err
			return
		}
		defer conn.Close()
		buf := make([]byte, 4096)
		n, err := conn.Read(buf)
		if err != nil {
			done <- err
			return
		}
		msgs, _, err := DecodeMessages(buf[:n], CodecOptions{LocalKey: localKey, Prefixed: true})
		if err != nil || len(msgs) != 1 || msgs[0].Protocol != ProtoHelloRequest {
			done <- fmt.Errorf("bad hello: %+v err=%v", msgs, err)
			return
		}
		connectNonce := msgs[0].Random
		helloResp, _ := EncodeMessage(Message{
			Version:   VersionL01,
			Seq:       msgs[0].Seq,
			Random:    ackNonce,
			Timestamp: msgs[0].Timestamp,
			Protocol:  ProtoHelloResponse,
		}, CodecOptions{LocalKey: localKey, Prefixed: true})
		_, _ = conn.Write(helloResp)

		n, err = conn.Read(buf)
		if err != nil {
			done <- err
			return
		}
		opts := CodecOptions{LocalKey: localKey, Prefixed: true, ConnectNonce: connectNonce, AckNonce: &ackNonce}
		msgs, _, err = DecodeMessages(buf[:n], opts)
		if err != nil || len(msgs) != 1 {
			done <- fmt.Errorf("bad L01 rpc: %+v err=%v", msgs, err)
			return
		}
		var env map[string]any
		_ = json.Unmarshal(msgs[0].Payload, &env)
		dps, _ := env["dps"].(map[string]any)
		req, _ := extractRPCFromDPS(dps, "101")
		id, _ := anyToInt(req["id"])
		replyInner, _ := json.Marshal(map[string]any{"id": id, "result": []string{"ok"}})
		replyEnv, _ := json.Marshal(map[string]any{"t": time.Now().Unix(), "dps": map[string]any{"102": string(replyInner)}})
		replyWire, _ := EncodeMessage(Message{
			Version:   VersionL01,
			Seq:       msgs[0].Seq,
			Random:    msgs[0].Random,
			Timestamp: uint32(time.Now().Unix()),
			Protocol:  ProtoGeneralResponse,
			Payload:   replyEnv,
		}, opts)
		_, _ = conn.Write(replyWire)
		done <- nil
	}()

	client := NewLocalTCPClient(ln.Addr().String(), localKey, VersionL01)
	if err := client.Connect(t.Context()); err != nil {
		t.Fatalf("LocalTCPClient.Connect: %v", err)
	}
	defer client.Close()
	res, err := client.SendRPC(t.Context(), "find_me", nil)
	if err != nil {
		t.Fatalf("LocalTCPClient.SendRPC: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("mock L01 server error: %v", err)
	}
	if !strings.Contains(string(res.Result), `"ok"`) {
		t.Fatalf("L01 result: got: %s, want [\"ok\"]", string(res.Result))
	}
}

// --- Minimal MQTT 3.1.1 test helpers (zero external client dependency) ---

func encodeMQTTVarInt(n int) []byte {
	var out []byte
	for {
		b := byte(n % 128)
		n /= 128
		if n > 0 {
			b |= 0x80
		}
		out = append(out, b)
		if n == 0 {
			break
		}
	}
	return out
}

func encodeMQTTString(s string) []byte {
	out := make([]byte, 2+len(s))
	binary.BigEndian.PutUint16(out[:2], uint16(len(s)))
	copy(out[2:], s)
	return out
}

func sendRawMQTTConnect(t *testing.T, conn net.Conn, clientID, user, pass string) {
	t.Helper()
	var body []byte
	body = append(body, encodeMQTTString("MQTT")...)
	body = append(body, 4, 0xC2, 0, 60) // v3.1.1, user+pass+cleanSession, keepalive=60
	body = append(body, encodeMQTTString(clientID)...)
	body = append(body, encodeMQTTString(user)...)
	body = append(body, encodeMQTTString(pass)...)
	pkt := append([]byte{0x10}, encodeMQTTVarInt(len(body))...)
	pkt = append(pkt, body...)
	if _, err := conn.Write(pkt); err != nil {
		t.Fatal(err)
	}
	ack := make([]byte, 4)
	if _, err := io.ReadFull(conn, ack); err != nil {
		t.Fatalf("read CONNACK: %v", err)
	}
	if ack[0] != 0x20 || ack[3] != 0x00 {
		t.Fatalf("CONNACK rejected: %x", ack)
	}
}

func sendRawMQTTSubscribe(t *testing.T, conn net.Conn, pktID uint16, topic string) {
	t.Helper()
	var body []byte
	body = append(body, byte(pktID>>8), byte(pktID))
	body = append(body, encodeMQTTString(topic)...)
	body = append(body, 0x00) // QoS 0
	pkt := append([]byte{0x82}, encodeMQTTVarInt(len(body))...)
	pkt = append(pkt, body...)
	if _, err := conn.Write(pkt); err != nil {
		t.Fatal(err)
	}
	suback := make([]byte, 5)
	if _, err := io.ReadFull(conn, suback); err != nil {
		t.Fatalf("read SUBACK: %v", err)
	}
}

func sendRawMQTTPublish(t *testing.T, conn net.Conn, topic string, payload []byte) {
	t.Helper()
	var body []byte
	body = append(body, encodeMQTTString(topic)...)
	body = append(body, payload...)
	pkt := append([]byte{0x30}, encodeMQTTVarInt(len(body))...)
	pkt = append(pkt, body...)
	if _, err := conn.Write(pkt); err != nil {
		t.Fatal(err)
	}
}

func readRawMQTTPublish(conn net.Conn) (string, []byte, error) {
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var hdr [1]byte
	if _, err := io.ReadFull(conn, hdr[:]); err != nil {
		return "", nil, err
	}
	mult, remLen := 1, 0
	for {
		var b [1]byte
		if _, err := io.ReadFull(conn, b[:]); err != nil {
			return "", nil, err
		}
		remLen += int(b[0]&0x7F) * mult
		if b[0]&0x80 == 0 {
			break
		}
		mult *= 128
	}
	body := make([]byte, remLen)
	if _, err := io.ReadFull(conn, body); err != nil {
		return "", nil, err
	}
	tLen := int(binary.BigEndian.Uint16(body[:2]))
	topic := string(body[2 : 2+tLen])
	return topic, body[2+tLen:], nil
}

func pickFreeTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return port
}

func pickFreeUDPPort(t *testing.T) int {
	t.Helper()
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := pc.LocalAddr().(*net.UDPAddr).Port
	_ = pc.Close()
	return port
}
