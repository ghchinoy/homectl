package roborock

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"testing"
	"time"
)

func TestOnboardingHelpers(t *testing.T) {
	if got := POSIXTZFromIANA("America/Denver"); got != "MST7MDT,M3.2.0,M11.1.0" {
		t.Fatalf("POSIXTZFromIANA: got: %q, want: %q", got, "MST7MDT,M3.2.0,M11.1.0")
	}
	if got := CountryFromIANA("Europe/Vienna"); got != "eu" {
		t.Fatalf("CountryFromIANA: got: %q, want: %q", got, "eu")
	}
	if got := SSIDPreKey("roborock.vacuum.a10", "aa:bb:cc:dd:5f:1d"); got != "cuum-a10_miap5F1D"[1:] {
		t.Fatalf("SSIDPreKey: got: %q", got)
	}
	tokenR, err := SanitizeStackServer("api-r.example.com", 443)
	if err != nil || tokenR != "r.example.com/" {
		t.Fatalf("SanitizeStackServer: got: %q err=%v, want: %q", tokenR, err, "r.example.com/")
	}
	apiURL, err := NormalizeAPIBaseURL("r.example.com", 443)
	if err != nil || apiURL != "https://api-r.example.com" {
		t.Fatalf("NormalizeAPIBaseURL: got: %q err=%v, want: %q", apiURL, err, "https://api-r.example.com")
	}
}

func TestSendCfgWiFiMockAP(t *testing.T) {
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer pc.Close()
	port := pc.LocalAddr().(*net.UDPAddr).Port

	sessionKey := "1234567890abcdef"
	gotCfg := make(chan map[string]any, 1)
	errCh := make(chan error, 1)

	go func() {
		buf := make([]byte, 4096)
		_ = pc.SetDeadline(time.Now().Add(3 * time.Second))
		// Step 1: Receive hello (cmd 16)
		n, peer, err := pc.ReadFrom(buf)
		if err != nil {
			errCh <- err
			return
		}
		cmdID, enc, err := ParseOnboardingFrame(buf[:n])
		if err != nil || cmdID != 16 {
			errCh <- fmt.Errorf("hello frame: cmd=%d err=%v", cmdID, err)
			return
		}
		plain, err := decryptAESECB(enc, []byte(DefaultCfgWiFiPreKey))
		if err != nil {
			errCh <- err
			return
		}
		var hello struct {
			Method string `json:"method"`
			Params struct {
				Key string `json:"key"`
			} `json:"params"`
		}
		if err := json.Unmarshal(plain, &hello); err != nil || hello.Method != "hello" {
			errCh <- fmt.Errorf("bad hello body: %s", string(plain))
			return
		}
		blk, _ := pem.Decode([]byte(hello.Params.Key))
		pubAny, err := x509.ParsePKIXPublicKey(blk.Bytes)
		if err != nil {
			errCh <- err
			return
		}
		pub := pubAny.(*rsa.PublicKey)
		respPlain, _ := json.Marshal(map[string]any{"id": 1, "params": map[string]string{"key": sessionKey}})
		encResp, err := rsa.EncryptPKCS1v15(rand.Reader, pub, respPlain)
		if err != nil {
			errCh <- err
			return
		}
		_, _ = pc.WriteTo(BuildOnboardingFrame(encResp, 16), peer)

		// Step 2: Receive cfgwifi (cmd 1)
		n, _, err = pc.ReadFrom(buf)
		if err != nil {
			errCh <- err
			return
		}
		cmdID, encWifi, err := ParseOnboardingFrame(buf[:n])
		if err != nil || cmdID != 1 {
			errCh <- fmt.Errorf("cfgwifi frame: cmd=%d err=%v", cmdID, err)
			return
		}
		wifiPlain, err := decryptAESECB(encWifi, []byte(sessionKey))
		if err != nil {
			errCh <- err
			return
		}
		var parsed map[string]any
		_ = json.Unmarshal(wifiPlain, &parsed)
		gotCfg <- parsed
		errCh <- nil
	}()

	err = SendCfgWiFi(t.Context(), CfgWiFiOptions{
		APHost:        "127.0.0.1",
		APPort:        port,
		SSID:          "HomeWiFi24",
		Password:      "SecretPass",
		TokenR:        "r.example.com/",
		Timezone:      "America/Denver",
		CountryDomain: "r.example.com/",
		Timeout:       2 * time.Second,
	})
	if err != nil {
		t.Fatalf("SendCfgWiFi failed: %v", err)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("mock AP error: %v", err)
	}
	cfg := <-gotCfg
	if cfg["ssid"] != "HomeWiFi24" || cfg["country_domain"] != "r.example.com/" {
		t.Fatalf("cfgwifi payload mismatch: got: %+v", cfg)
	}
	tok, _ := cfg["token"].(map[string]any)
	if tok["r"] != "r.example.com/" || tok["cst"] != "MST7MDT,M3.2.0,M11.1.0" {
		t.Fatalf("cfgwifi token mismatch: got: %+v", tok)
	}
}
