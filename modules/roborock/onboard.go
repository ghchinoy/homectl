package roborock

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultCfgWiFiHost   = "192.168.8.1"
	DefaultCfgWiFiPort   = 55559
	DefaultCfgWiFiPreKey = "6433df70f5a3a42e"
	DefaultCfgWiFiUID    = "1234567890"
)

var ianaToPOSIX = map[string]string{
	"America/New_York":    "EST5EDT,M3.2.0,M11.1.0",
	"America/Chicago":     "CST6CDT,M3.2.0,M11.1.0",
	"America/Denver":      "MST7MDT,M3.2.0,M11.1.0",
	"America/Los_Angeles": "PST8PDT,M3.2.0,M11.1.0",
	"America/Phoenix":     "MST7",
	"America/Anchorage":   "AKST9AKDT,M3.2.0,M11.1.0",
	"Pacific/Honolulu":    "HST10",
	"America/Toronto":     "EST5EDT,M3.2.0,M11.1.0",
	"America/Vancouver":   "PST8PDT,M3.2.0,M11.1.0",
	"Europe/London":       "GMT0BST,M3.5.0/1,M10.5.0",
	"Europe/Berlin":       "CET-1CEST,M3.5.0,M10.5.0/3",
	"Europe/Paris":        "CET-1CEST,M3.5.0,M10.5.0/3",
	"Europe/Amsterdam":    "CET-1CEST,M3.5.0,M10.5.0/3",
	"Europe/Zurich":       "CET-1CEST,M3.5.0,M10.5.0/3",
	"Europe/Moscow":       "MSK-3",
	"Asia/Shanghai":       "CST-8",
	"Asia/Tokyo":          "JST-9",
	"Asia/Kolkata":        "IST-5:30",
	"Australia/Sydney":    "AEST-10AEDT,M10.1.0,M4.1.0/3",
}

var ianaToCountry = map[string]string{
	"America/New_York":    "us",
	"America/Chicago":     "us",
	"America/Denver":      "us",
	"America/Los_Angeles": "us",
	"America/Phoenix":     "us",
	"America/Anchorage":   "us",
	"Pacific/Honolulu":    "us",
	"America/Toronto":     "us",
	"America/Vancouver":   "us",
	"Europe/London":       "eu",
	"Europe/Berlin":       "eu",
	"Europe/Paris":        "eu",
	"Europe/Amsterdam":    "eu",
	"Europe/Zurich":       "eu",
	"Europe/Moscow":       "ru",
	"Asia/Shanghai":       "cn",
	"Asia/Tokyo":          "jp",
	"Asia/Kolkata":        "in",
	"Australia/Sydney":    "au",
}

// POSIXTZFromIANA maps an IANA timezone to the POSIX TZ string expected by vacuum firmware.
func POSIXTZFromIANA(iana string) string {
	return ianaToPOSIX[strings.TrimSpace(iana)]
}

// CountryFromIANA maps an IANA timezone to Roborock's regional country domain code.
func CountryFromIANA(iana string) string {
	cleaned := strings.TrimSpace(iana)
	if v, ok := ianaToCountry[cleaned]; ok {
		return v
	}
	if strings.HasPrefix(cleaned, "Europe/") {
		return "eu"
	}
	return "us"
}

// SSIDPreKey derives the alternate 16-char cfgwifi pre-key from model and vacuum AP MAC address.
func SSIDPreKey(model, mac string) string {
	parts := strings.FieldsFunc(mac, func(r rune) bool { return r == ':' || r == '-' })
	if len(parts) < 2 {
		return ""
	}
	var suffix strings.Builder
	for _, p := range parts[len(parts)-2:] {
		v, err := strconv.ParseUint(p, 16, 8)
		if err != nil {
			return ""
		}
		fmt.Fprintf(&suffix, "%02X", v)
	}
	ssid := fmt.Sprintf("%s_miap%s", strings.ReplaceAll(strings.TrimSpace(model), ".", "-"), suffix.String())
	if len(ssid) <= 16 {
		return ssid
	}
	return ssid[len(ssid)-16:]
}

// SanitizeStackServer normalizes a server host[:port] into the onboarding token.r value (<=32 chars, trailing '/').
func SanitizeStackServer(raw string, defaultPort int) (string, error) {
	host, port, err := parseServerTarget(raw, defaultPort)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(strings.ToLower(host), "api-") {
		host = host[4:]
	}
	authority := host
	if port > 0 && port != 443 {
		authority = fmt.Sprintf("%s:%d", host, port)
	}
	tokenR := authority + "/"
	if len(tokenR) > MaxTokenRLen {
		return "", fmt.Errorf("roborock: onboarding token.r %q exceeds %d-char limit (%d chars)", tokenR, MaxTokenRLen, len(tokenR))
	}
	return tokenR, nil
}

// NormalizeAPIBaseURL ensures the stack URL has the "api-" host prefix and https:// scheme.
func NormalizeAPIBaseURL(raw string, defaultPort int) (string, error) {
	host, port, err := parseServerTarget(raw, defaultPort)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(strings.ToLower(host), "api-") {
		host = "api-" + host
	}
	if port <= 0 || port == 443 {
		return "https://" + host, nil
	}
	return fmt.Sprintf("https://%s:%d", host, port), nil
}

func parseServerTarget(raw string, defaultPort int) (string, int, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", 0, errors.New("roborock: server host is required")
	}
	if !strings.Contains(v, "://") {
		v = "https://" + v
	}
	u, err := url.Parse(v)
	if err != nil {
		return "", 0, fmt.Errorf("roborock: invalid server URL %q: %w", raw, err)
	}
	host := strings.Trim(u.Hostname(), "/")
	if host == "" {
		return "", 0, errors.New("roborock: server hostname is empty")
	}
	port := defaultPort
	if ps := u.Port(); ps != "" {
		p, err := strconv.Atoi(ps)
		if err != nil || p <= 0 || p > 65535 {
			return "", 0, fmt.Errorf("roborock: invalid server port %q", ps)
		}
		port = p
	}
	if port <= 0 {
		port = 443
	}
	return host, port, nil
}

// BuildOnboardingFrame builds a Roborock UDP onboarding packet ("1.0" + 0x00000001 + cmdID + len + payload + CRC32).
func BuildOnboardingFrame(payload []byte, cmdID uint16) []byte {
	buf := make([]byte, 0, 11+len(payload)+4)
	buf = append(buf, '1', '.', '0', 0, 0, 0, 1)
	buf = append(buf, byte(cmdID>>8), byte(cmdID))
	buf = append(buf, byte(len(payload)>>8), byte(len(payload)))
	buf = append(buf, payload...)
	sum := crc32.ChecksumIEEE(buf)
	var b4 [4]byte
	binary.BigEndian.PutUint32(b4[:], sum)
	buf = append(buf, b4[:]...)
	return buf
}

// ParseOnboardingFrame validates and extracts (cmdID, payload) from a UDP onboarding packet.
func ParseOnboardingFrame(pkt []byte) (uint16, []byte, error) {
	if len(pkt) < 15 || !bytes.Equal(pkt[:3], []byte("1.0")) {
		return 0, nil, errors.New("roborock: invalid onboarding UDP frame")
	}
	cmdID := binary.BigEndian.Uint16(pkt[7:9])
	ln := int(binary.BigEndian.Uint16(pkt[9:11]))
	if len(pkt) < 11+ln+4 {
		return 0, nil, errors.New("roborock: truncated onboarding UDP frame")
	}
	wantCRC := crc32.ChecksumIEEE(pkt[:11+ln])
	gotCRC := binary.BigEndian.Uint32(pkt[11+ln : 11+ln+4])
	if gotCRC != wantCRC {
		return 0, nil, fmt.Errorf("%w: got 0x%08x want 0x%08x", ErrChecksum, gotCRC, wantCRC)
	}
	return cmdID, pkt[11 : 11+ln], nil
}

// CfgWiFiOptions configures the UDP hello + AES cfgwifi onboarding exchange with a vacuum AP.
type CfgWiFiOptions struct {
	APHost        string // defaults to 192.168.8.1
	APPort        int    // defaults to 55559
	PreKey        string // defaults to DefaultCfgWiFiPreKey
	AltPreKey     string // optional MAC-derived SSIDPreKey fallback
	SSID          string
	Password      string
	TokenR        string // e.g. "r.example.com/"
	Timezone      string // e.g. "America/Denver"
	CST           string // POSIX TZ string
	CountryDomain string // e.g. "us" or "r.example.com/" for Saros 10R camera
	UID           string // defaults to DefaultCfgWiFiUID
	Timeout       time.Duration
}

// SendCfgWiFi performs the 2-step UDP onboarding handshake against the vacuum's temporary Wi-Fi hotspot.
func SendCfgWiFi(ctx context.Context, opts CfgWiFiOptions) error {
	if opts.APHost == "" {
		opts.APHost = DefaultCfgWiFiHost
	}
	if opts.APPort <= 0 {
		opts.APPort = DefaultCfgWiFiPort
	}
	if opts.PreKey == "" {
		opts.PreKey = DefaultCfgWiFiPreKey
	}
	if opts.UID == "" {
		opts.UID = DefaultCfgWiFiUID
	}
	if opts.Timezone == "" {
		opts.Timezone = "America/New_York"
	}
	if opts.CST == "" {
		opts.CST = POSIXTZFromIANA(opts.Timezone)
		if opts.CST == "" {
			opts.CST = "EST5EDT,M3.2.0,M11.1.0"
		}
	}
	if opts.CountryDomain == "" {
		opts.CountryDomain = CountryFromIANA(opts.Timezone)
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 3 * time.Second
	}
	if opts.SSID == "" || opts.TokenR == "" {
		return errors.New("roborock: ssid and token.r are required for cfgwifi")
	}

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return err
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})

	addr := net.JoinHostPort(opts.APHost, strconv.Itoa(opts.APPort))
	var d net.Dialer
	conn, err := d.DialContext(ctx, "udp4", addr)
	if err != nil {
		return fmt.Errorf("roborock: dial vacuum AP %s: %w", addr, err)
	}
	defer conn.Close()

	preKeys := []string{opts.PreKey}
	if opts.AltPreKey != "" && opts.AltPreKey != opts.PreKey {
		preKeys = append(preKeys, opts.AltPreKey)
	}

	var sessionKey string
	var lastErr error
	for _, pk := range preKeys {
		helloBody, err := json.Marshal(map[string]any{
			"id":     1,
			"method": "hello",
			"params": map[string]any{
				"app_ver": 1,
				"key":     string(pubPEM),
			},
		})
		if err != nil {
			return err
		}
		encHello, err := encryptAESECB(helloBody, []byte(pk))
		if err != nil {
			lastErr = err
			continue
		}
		pkt := BuildOnboardingFrame(encHello, 16)
		_ = conn.SetDeadline(time.Now().Add(opts.Timeout))
		if _, err := conn.Write(pkt); err != nil {
			lastErr = err
			continue
		}
		buf := make([]byte, 4096)
		n, err := conn.Read(buf)
		if err != nil {
			lastErr = fmt.Errorf("hello reply timeout/error (check that you joined the vacuum Wi-Fi AP): %w", err)
			continue
		}
		_, encResp, err := ParseOnboardingFrame(buf[:n])
		if err != nil {
			lastErr = err
			continue
		}
		plainResp, err := rsaDecryptBlocksPKCS1v15(encResp, priv)
		if err != nil {
			lastErr = err
			continue
		}
		var helloResp struct {
			Params struct {
				Key string `json:"key"`
			} `json:"params"`
		}
		if err := json.Unmarshal(plainResp, &helloResp); err != nil || len(helloResp.Params.Key) != 16 {
			lastErr = fmt.Errorf("invalid session key in hello reply: %q", string(plainResp))
			continue
		}
		sessionKey = helloResp.Params.Key
		break
	}
	if sessionKey == "" {
		return fmt.Errorf("roborock: onboarding hello failed: %w", lastErr)
	}

	wifiBody, err := json.Marshal(map[string]any{
		"u":      opts.UID,
		"ssid":   opts.SSID,
		"passwd": opts.Password,
		"token": map[string]string{
			"r":   opts.TokenR,
			"tz":  opts.Timezone,
			"s":   "S_" + randomHex(8),
			"cst": opts.CST,
			"t":   "T_" + randomHex(8),
		},
		"country_domain": opts.CountryDomain,
	})
	if err != nil {
		return err
	}
	encWifi, err := encryptAESECB(wifiBody, []byte(sessionKey))
	if err != nil {
		return err
	}
	wifiPkt := BuildOnboardingFrame(encWifi, 1)
	_ = conn.SetDeadline(time.Now().Add(opts.Timeout))
	if _, err := conn.Write(wifiPkt); err != nil {
		return fmt.Errorf("roborock: send cfgwifi packet: %w", err)
	}
	// Best-effort read of ack (some models reboot/disconnect AP immediately on receipt).
	ackBuf := make([]byte, 1024)
	_, _ = conn.Read(ackBuf)
	return nil
}

func rsaDecryptBlocksPKCS1v15(payload []byte, priv *rsa.PrivateKey) ([]byte, error) {
	blockSize := priv.Size()
	if len(payload) == 0 || len(payload)%blockSize != 0 {
		return nil, fmt.Errorf("roborock: invalid rsa block length %d (blockSize=%d)", len(payload), blockSize)
	}
	var out []byte
	for off := 0; off < len(payload); off += blockSize {
		chunk, err := rsa.DecryptPKCS1v15(rand.Reader, priv, payload[off:off+blockSize])
		if err != nil {
			return nil, err
		}
		out = append(out, chunk...)
	}
	return out, nil
}

// PreflightResult summarizes DNS, TLS, HTTPS, MQTT, and optional camera bootstrap checks.
type PreflightResult struct {
	OK               bool     `json:"ok"`
	APIBaseURL       string   `json:"api_base_url"`
	TokenR           string   `json:"token_r"`
	MQTTAddr         string   `json:"mqtt_addr"`
	TURNMode         string   `json:"turn_mode,omitempty"`
	TURNRunning      bool     `json:"turn_running,omitempty"`
	Checks           []string `json:"checks"`
	Errors           []string `json:"errors,omitempty"`
}

// Preflight verifies DNS resolution, TLS certificate trust, /admin/api/status reachability,
// MQTT TLS handshake, and (when cameraDomain is set) the <=15-char /fwapi/createca contract.
func Preflight(ctx context.Context, serverArg, model, cameraDomain string, defaultHTTPSPort int, allowInsecure bool) (*PreflightResult, error) {
	if defaultHTTPSPort <= 0 {
		defaultHTTPSPort = 443
	}
	tokenR, err := SanitizeStackServer(serverArg, defaultHTTPSPort)
	if err != nil {
		return nil, err
	}
	apiBase, err := NormalizeAPIBaseURL(serverArg, defaultHTTPSPort)
	if err != nil {
		return nil, err
	}
	res := &PreflightResult{
		OK:         true,
		APIBaseURL: apiBase,
		TokenR:     tokenR,
	}

	u, _ := url.Parse(apiBase)
	apiHost := u.Hostname()
	apiPort := 443
	if u.Port() != "" {
		apiPort, _ = strconv.Atoi(u.Port())
	}

	// 1. HTTPS TLS probe
	if err := probeTLS(ctx, apiHost, apiPort, allowInsecure); err != nil {
		res.OK = false
		res.Errors = append(res.Errors, fmt.Sprintf("HTTPS TLS (%s:%d): %v", apiHost, apiPort, err))
	} else {
		res.Checks = append(res.Checks, fmt.Sprintf("HTTPS TLS ok (%s:%d)", apiHost, apiPort))
	}

	// 2. GET /admin/api/status
	httpClient := &http.Client{
		Timeout: 8 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: allowInsecure},
		},
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+"/admin/api/status", nil)
	resp, err := httpClient.Do(req)
	mqttPort := 8881
	if err != nil {
		res.OK = false
		res.Errors = append(res.Errors, fmt.Sprintf("GET /admin/api/status: %v", err))
	} else {
		defer resp.Body.Close()
		var status struct {
			OK                    bool `json:"ok"`
			AdvertisedMQTTTLSPort int  `json:"advertised_mqtt_tls_port"`
			TURN                  struct {
				Mode    string `json:"mode"`
				Host    string `json:"host"`
				Port    int    `json:"port"`
				Running bool   `json:"running"`
			} `json:"turn"`
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		if err := json.Unmarshal(raw, &status); err != nil || !status.OK {
			res.OK = false
			res.Errors = append(res.Errors, "GET /admin/api/status did not return ok=true")
		} else {
			res.Checks = append(res.Checks, "Admin status endpoint ok")
			if status.AdvertisedMQTTTLSPort > 0 {
				mqttPort = status.AdvertisedMQTTTLSPort
			}
			res.TURNMode = status.TURN.Mode
			res.TURNRunning = status.TURN.Running
		}
	}

	// 3. MQTT TLS probe
	res.MQTTAddr = fmt.Sprintf("%s:%d", apiHost, mqttPort)
	if err := probeTLS(ctx, apiHost, mqttPort, allowInsecure); err != nil {
		res.OK = false
		res.Errors = append(res.Errors, fmt.Sprintf("MQTT TLS (%s): %v", res.MQTTAddr, err))
	} else {
		res.Checks = append(res.Checks, fmt.Sprintf("MQTT TLS ok (%s)", res.MQTTAddr))
	}

	// 4. Optional Camera <=15-char bootstrap probe
	if strings.TrimSpace(cameraDomain) != "" {
		camInput := strings.TrimRight(strings.TrimSpace(cameraDomain), "/") + "/"
		eps, err := CameraBootstrapEndpoints(model, tokenR, camInput)
		if err != nil {
			res.OK = false
			res.Errors = append(res.Errors, err.Error())
		} else {
			for _, ep := range eps {
				if err := probeTLS(ctx, ep.Host, ep.Port, false); err != nil {
					res.OK = false
					res.Errors = append(res.Errors, fmt.Sprintf("Camera TLS (%s:%d): %v", ep.Host, ep.Port, err))
					continue
				}
				cReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, ep.URL, strings.NewReader("{}"))
				cReq.Header.Set("Content-Type", "application/json")
				cResp, cErr := (&http.Client{Timeout: 8 * time.Second}).Do(cReq)
				if cErr != nil {
					res.OK = false
					res.Errors = append(res.Errors, fmt.Sprintf("Camera createca (%s): %v", ep.URL, cErr))
					continue
				}
				body, _ := io.ReadAll(io.LimitReader(cResp.Body, 64<<10))
				_ = cResp.Body.Close()
				if cResp.StatusCode != 200 || !strings.Contains(string(body), `"turn:`) {
					res.OK = false
					res.Errors = append(res.Errors, fmt.Sprintf("Camera createca (%s) did not return a TURN URL", ep.URL))
				} else {
					res.Checks = append(res.Checks, fmt.Sprintf("Camera bootstrap ok (%s)", ep.URL))
				}
			}
		}
	}

	return res, nil
}

func probeTLS(ctx context.Context, host string, port int, allowInsecure bool) error {
	dialer := &tls.Dialer{
		NetDialer: &net.Dialer{Timeout: 6 * time.Second},
		Config: &tls.Config{
			ServerName:         host,
			InsecureSkipVerify: allowInsecure,
		},
	}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return err
	}
	return conn.Close()
}
