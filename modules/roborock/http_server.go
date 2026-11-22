package roborock

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/ghchinoy/homectl/modules/core"
)

const maxHTTPBodyBytes = 1 << 20 // 1 MiB bounded request body cap

var (
	safeMetaValueRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)
	decimalDigitsRe = regexp.MustCompile(`^[0-9]{1,32}$`)
	hexDigitsRe     = regexp.MustCompile(`^[0-9A-Fa-f]{1,128}$`)
)

// HTTPServerConfig configures the local Roborock HTTPS bootstrap & firmware API server.
type HTTPServerConfig struct {
	BindHost          string
	Port              int
	AdvertisedHost    string // stack_fqdn, e.g. "api-r.example.com"
	AdvertisedPort    int    // optional override when behind reverse proxy (defaults to Port)
	AdvertisedMQTT    string // e.g. "api-r.example.com"
	AdvertisedMQTTPort int   // e.g. 8881
	Region            string // "us", "eu", "cn", "ru"
	Timezone          string // IANA timezone, e.g. "America/Denver"
	AllowLegacyTLS    bool
	DisableTLS        bool // true only when external proxy terminates TLS or in plaintext unit tests
}

// HTTPServer implements the local Roborock HTTPS bootstrap, rrcheck, device_info, and TURN endpoints.
type HTTPServer struct {
	cfg       HTTPServerConfig
	keys      *KeyStore
	creds     *CredentialStore
	telemetry *TelemetryHub
	turn      *TURNRelay
	certs     *MultiSNICertManager
	logger    core.Logger
	server    *http.Server
	mux       *http.ServeMux
	appMux    http.Handler // optional Phase 4 HA/LocalRock app API handler
	ln        net.Listener
}

// NewHTTPServer creates a configured Roborock HTTPS server backed by a dedicated http.ServeMux.
func NewHTTPServer(
	cfg HTTPServerConfig,
	keys *KeyStore,
	creds *CredentialStore,
	telemetry *TelemetryHub,
	turn *TURNRelay,
	certs *MultiSNICertManager,
	logger core.Logger,
) *HTTPServer {
	if cfg.BindHost == "" {
		cfg.BindHost = "0.0.0.0"
	}
	if cfg.Port <= 0 {
		cfg.Port = 443
	}
	if cfg.AdvertisedPort <= 0 {
		cfg.AdvertisedPort = cfg.Port
	}
	if cfg.AdvertisedMQTT == "" {
		cfg.AdvertisedMQTT = cfg.AdvertisedHost
	}
	if cfg.AdvertisedMQTTPort <= 0 {
		cfg.AdvertisedMQTTPort = 8881
	}
	if cfg.Region == "" {
		cfg.Region = "us"
	}
	if cfg.Timezone == "" {
		cfg.Timezone = defaultTimezoneForRegion(cfg.Region)
	}
	if logger == nil {
		logger = core.NewNoOpLogger()
	}

	s := &HTTPServer{
		cfg:       cfg,
		keys:      keys,
		creds:     creds,
		telemetry: telemetry,
		turn:      turn,
		certs:     certs,
		logger:    logger,
		mux:       http.NewServeMux(),
	}
	s.mux.HandleFunc("/", s.handleAll)
	return s
}

// SetAppAPIHandler attaches an optional Phase 4 handler (HA / LocalRock / App login & inventory routes).
func (s *HTTPServer) SetAppAPIHandler(h http.Handler) {
	s.appMux = h
}

// Handler returns the HTTP handler for httptest or embedded serving.
func (s *HTTPServer) Handler() http.Handler {
	return s.mux
}

// Start binds the TCP/TLS listener and serves requests in a background goroutine.
func (s *HTTPServer) Start(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.BindHost, s.cfg.Port)
	rawLn, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("roborock: listen HTTP %s: %w", addr, err)
	}

	ln := rawLn
	if !s.cfg.DisableTLS {
		if s.certs == nil {
			_ = rawLn.Close()
			return fmt.Errorf("roborock: TLS enabled on %s but no certificate manager provided", addr)
		}
		ln = tls.NewListener(rawLn, s.certs.TLSConfig(s.cfg.AllowLegacyTLS))
	}
	s.ln = ln
	s.server = &http.Server{
		Handler:           s.mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		if err := s.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.logger.Printf("roborock: HTTPS server error: %v", err)
		}
	}()
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.Shutdown(shutCtx)
	}()

	scheme := "https"
	if s.cfg.DisableTLS {
		scheme = "http"
	}
	s.logger.Printf("roborock: %s bootstrap server listening on %s (api=%s)", scheme, ln.Addr().String(), s.apiURL(""))
	return nil
}

// Addr returns the bound listener address, or empty if not started.
func (s *HTTPServer) Addr() string {
	if s.ln == nil {
		return ""
	}
	return s.ln.Addr().String()
}

// Shutdown gracefully stops the HTTPS server.
func (s *HTTPServer) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}
	return s.server.Shutdown(ctx)
}

// StripRoborockPrefix normalizes injected paths like "/.roborock.com/region" or "/iot.roborock.com/fwapi/createca".
func StripRoborockPrefix(rawPath string) string {
	p := rawPath
	if p == "" {
		return "/"
	}
	for _, prefix := range []string{"/.roborock.com", "/iot.roborock.com"} {
		if strings.HasPrefix(p, prefix) {
			p = strings.TrimPrefix(p, prefix)
			if p == "" {
				return "/"
			}
		}
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	return p
}

func (s *HTTPServer) apiURL(hostOverride string) string {
	host := s.cfg.AdvertisedHost
	port := s.cfg.AdvertisedPort
	if hostOverride != "" {
		h, p := splitHostPort(hostOverride)
		if h != "" {
			host = h
		}
		if p > 0 {
			port = p
		}
	}
	if port == 443 || port <= 0 {
		return "https://" + host
	}
	return fmt.Sprintf("https://%s:%d", host, port)
}

func (s *HTTPServer) mqttURL(hostOverride string) string {
	host := s.cfg.AdvertisedMQTT
	if hostOverride != "" {
		if h, _ := splitHostPort(hostOverride); h != "" {
			host = h
		}
	}
	return fmt.Sprintf("ssl://%s:%d", host, s.cfg.AdvertisedMQTTPort)
}

func (s *HTTPServer) handleAll(w http.ResponseWriter, r *http.Request) {
	rawPath := r.URL.Path
	cleanPath := StripRoborockPrefix(rawPath)
	rawQuery := r.URL.RawQuery

	// Read bounded body once so both signature capture and form/multipart/JSON parsers can inspect it.
	rawBody, err := io.ReadAll(io.LimitReader(r.Body, maxHTTPBodyBytes))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "msg": "read body failed"})
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(rawBody))

	queryParams := r.URL.Query()
	bodyParams := parseBodyParams(r, rawBody)

	did := firstParam(queryParams, bodyParams, "did", "d", "duid", "device_id", "deviceId")
	model := firstParam(queryParams, bodyParams, "pid", "m", "model")

	// Capture V1 query signature & V2 header signature samples before route handling.
	s.captureSignatures(r.Context(), r, rawPath, cleanPath, rawQuery, rawBody, did, model)

	trimmed := strings.TrimRight(cleanPath, "/")

	// Admin / Onboarding Status endpoints
	switch trimmed {
	case "/admin/api/status":
		s.handleAdminStatus(w)
		return
	case "/admin/api/devices":
		s.handleAdminDevices(w)
		return
	}

	// Firmware Bootstrap, Time, Location, RRCheck, Device Info, and Camera TURN routes
	switch {
	case trimmed == "" || trimmed == "/region" || trimmed == "/api/region" || trimmed == "/b/region" || trimmed == "/api/b/region":
		s.handleRegion(w, r, did, queryParams)
		return
	case strings.Contains(trimmed, "nc") && (strings.Contains(trimmed, "prepare") || strings.HasSuffix(trimmed, "/nc")):
		s.handleNCPrepare(w, r, did, model, queryParams, bodyParams)
		return
	case trimmed == "/time/now" || trimmed == "/api/time/now" || strings.HasSuffix(trimmed, "/time/now"):
		writeJSON(w, http.StatusOK, wrapEnvelope(time.Now().Unix()))
		return
	case trimmed == "/location" || trimmed == "/api/location":
		writeJSON(w, http.StatusOK, wrapEnvelope(map[string]any{
			"country":  strings.ToUpper(s.cfg.Region),
			"region":   strings.ToLower(s.cfg.Region),
			"timezone": s.cfg.Timezone,
		}))
		return
	case isRRCheckPath(trimmed):
		s.handleRRCheck(w, trimmed, did)
		return
	case strings.HasPrefix(trimmed, "/devices/") && strings.HasSuffix(trimmed, "/info"):
		s.handleDeviceInfo(w, trimmed, string(rawBody))
		return
	case strings.HasSuffix(trimmed, "/fwapi/createca"):
		s.handleCreateCA(w, trimmed)
		return
	}

	// Optional Phase 4 HA / LocalRock App API routes
	if s.appMux != nil && isAppAPIPath(trimmed) {
		r.Body = io.NopCloser(bytes.NewReader(rawBody))
		s.appMux.ServeHTTP(w, r)
		return
	}

	// Catch-all acknowledgment matching upstream fallback contract.
	if s.telemetry != nil {
		s.telemetry.Emit(TelemetryEvent{
			Kind:    "http",
			DID:     did,
			Summary: fmt.Sprintf("%s %s (catchall)", r.Method, cleanPath),
		})
	}
	writeJSON(w, http.StatusOK, wrapEnvelope(map[string]any{
		"ok":    true,
		"route": cleanPath,
	}))
}

func (s *HTTPServer) captureSignatures(ctx context.Context, r *http.Request, rawPath, cleanPath, rawQuery string, rawBody []byte, did, model string) {
	if s.keys == nil || did == "" {
		return
	}
	captured := false
	if rawQuery != "" && strings.Contains(rawQuery, "signature=") {
		if added, err := s.keys.AddSignedQuery(did, rawQuery, model); err == nil && added {
			captured = true
			if s.telemetry != nil {
				s.telemetry.Emit(TelemetryEvent{
					Kind:    "key_recovery",
					DID:     did,
					Summary: "captured V1 query RSA signature sample",
				})
			}
		}
	}

	// Capture V2 header signatures (RSA-4096/SHA-384 used by Saros 10R a144 & Saros 20 a288).
	sigHdr := pickHeader(r.Header, "X-Signature", "Signature", "Sign")
	if sigHdr != "" {
		nonce := pickHeader(r.Header, "X-Nonce", "Nonce")
		ts := pickHeader(r.Header, "X-Timestamp", "Timestamp", "Ts")
		ver := strings.ToLower(pickHeader(r.Header, "X-Version", "Version", "Api-Version"))
		if ver == "" && len(sigHdr) > 600 {
			ver = "v2"
		}
		var bodyHash string
		if len(rawBody) > 0 {
			sum := sha256.Sum256(rawBody)
			bodyHash = hex.EncodeToString(sum[:])
		}
		hs := HeaderSample{
			Method:       r.Method,
			Path:         rawPath,
			Query:        rawQuery,
			Nonce:        nonce,
			TS:           ts,
			SignatureB64: sigHdr,
			BodySHA256:   bodyHash,
			Version:      ver,
		}
		if added, err := s.keys.AddHeaderSignature(did, hs, model); err == nil && added {
			captured = true
			if s.telemetry != nil {
				s.telemetry.Emit(TelemetryEvent{
					Kind:    "key_recovery",
					DID:     did,
					Summary: fmt.Sprintf("captured V2 header signature sample (%s %s)", r.Method, cleanPath),
				})
			}
		}
	}

	if captured {
		s.keys.MaybeRecoverAsync(ctx, did)
	}
}

func (s *HTTPServer) handleRegion(w http.ResponseWriter, r *http.Request, did string, queryParams url.Values) {
	hostOverride := queryParams.Get("host")
	apiURL := s.apiURL(hostOverride)
	mqttURL := s.mqttURL(hostOverride)

	payload := map[string]string{
		"apiUrl":   apiURL,
		"mqttUrl":  mqttURL,
		"api_url":  apiURL,
		"mqtt_url": mqttURL,
	}
	// Keep encrypted bootstrap payload within a single RSA-OAEP block (<=214 bytes) for firmware compatibility.
	if b, err := json.Marshal(payload); err == nil && len(b) > MaxBootstrapSingleBlockOAEP {
		payload = map[string]string{
			"apiUrl":  apiURL,
			"mqttUrl": mqttURL,
		}
	}

	if s.telemetry != nil {
		s.telemetry.Emit(TelemetryEvent{
			Kind:    "http",
			DID:     did,
			Summary: fmt.Sprintf("GET /region (encrypted=%v)", s.keys != nil && s.keys.GetPublicKey(did) != nil),
			Data:    map[string]any{"remote": r.RemoteAddr},
		})
	}

	if s.keys != nil && did != "" {
		if pub := s.keys.GetPublicKey(did); pub != nil {
			if enc, err := EncryptJSONOAEPSHA1B64(pub, payload); err == nil {
				writeJSON(w, http.StatusOK, map[string]any{"code": 200, "result": enc})
				return
			}
		}
	}
	writeJSON(w, http.StatusOK, wrapEnvelope(payload))
}

func (s *HTTPServer) handleNCPrepare(w http.ResponseWriter, r *http.Request, did, model string, queryParams, bodyParams url.Values) {
	session := firstParam(queryParams, bodyParams, "session", "s")
	if session == "" {
		session = "s_" + randomHex(8)
	}
	token := firstParam(queryParams, bodyParams, "token", "t")
	if token == "" {
		token = "t_" + randomHex(8)
	}
	remoteIP, _, _ := net.SplitHostPort(r.RemoteAddr)
	if remoteIP == "" {
		remoteIP = r.RemoteAddr
	}

	var localKey string
	if s.creds != nil && did != "" {
		dev, err := s.creds.EnsureDevice(DeviceCredential{
			DID:            did,
			Model:          model,
			LocalKeySource: "onboarding_nc",
			LastNCAt:       time.Now().UTC().Format(time.RFC3339),
			LastIP:         remoteIP,
		})
		if err == nil {
			localKey = dev.LocalKey
		}
	}
	if localKey == "" && s.creds != nil {
		localKey = s.creds.DefaultLocalKey()
	}

	minimalNC := map[string]string{
		"s": session,
		"t": token,
		"k": localKey,
	}
	if did != "" {
		minimalNC["d"] = did
	}

	if s.telemetry != nil {
		s.telemetry.Emit(TelemetryEvent{
			Kind:    "http",
			DID:     did,
			Summary: fmt.Sprintf("%s /nc (model=%s, encrypted=%v)", r.Method, model, s.keys != nil && s.keys.GetPublicKey(did) != nil),
		})
	}

	if s.keys != nil && did != "" {
		if pub := s.keys.GetPublicKey(did); pub != nil {
			if enc, err := EncryptJSONOAEPSHA1B64(pub, minimalNC); err == nil {
				writeJSON(w, http.StatusOK, map[string]any{"code": 200, "result": enc})
				return
			}
		}
	}
	writeJSON(w, http.StatusOK, wrapEnvelope(minimalNC))
}

func isRRCheckPath(trimmed string) bool {
	switch trimmed {
	case "/f/c", "/api/f/c", "/b/f/c", "/api/b/f/c":
		return true
	}
	return strings.HasPrefix(trimmed, "/devices/") && strings.HasSuffix(trimmed, "/sh")
}

func (s *HTTPServer) handleRRCheck(w http.ResponseWriter, trimmed, did string) {
	if did == "" && strings.HasPrefix(trimmed, "/devices/") {
		parts := strings.Split(strings.Trim(trimmed, "/"), "/")
		if len(parts) >= 2 {
			did = parts[1]
		}
	}
	var pubKey = func() *struct{ ok bool } { return nil }()
	_ = pubKey
	if s.keys != nil && did != "" {
		if pub := s.keys.GetPublicKey(did); pub != nil {
			grant, err := BuildRRCheckGrant(pub, s.cfg.Region, time.Now().Unix())
			if err == nil {
				if s.telemetry != nil {
					s.telemetry.Emit(TelemetryEvent{
						Kind:    "http",
						DID:     did,
						Summary: fmt.Sprintf("forged rrcheck grant (%s)", trimmed),
					})
				}
				if strings.HasSuffix(trimmed, "/sh") {
					writeJSON(w, http.StatusOK, map[string]any{
						"k":       grant.K,
						"d":       grant.D,
						"success": true,
						"code":    200,
						"msg":     "success",
						"data":    grant,
						"result":  grant,
					})
					return
				}
				writeJSON(w, http.StatusOK, grant)
				return
			}
		}
	}
	writeJSON(w, http.StatusOK, wrapEnvelope(map[string]any{"ok": true, "route": trimmed}))
}

func (s *HTTPServer) handleDeviceInfo(w http.ResponseWriter, trimmed, rawBody string) {
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	var urlDID string
	if len(parts) >= 2 {
		urlDID = parts[1]
	}

	outcome := s.processDeviceInfo(urlDID, rawBody)
	if s.telemetry != nil {
		s.telemetry.Emit(TelemetryEvent{
			Kind:    "http",
			DID:     urlDID,
			Summary: fmt.Sprintf("POST /devices/%s/info (%s)", urlDID, outcome),
		})
	}
	// Firmware checks HTTP 200 status only; preserve exact fallback envelope.
	writeJSON(w, http.StatusOK, wrapEnvelope(map[string]any{
		"ok":    true,
		"route": trimmed,
	}))
}

func (s *HTTPServer) processDeviceInfo(urlDID, rawBody string) string {
	vals, err := url.ParseQuery(rawBody)
	if err != nil {
		return "malformed_fields"
	}
	required := []string{"did", "featureset", "newfeatureset", "pid", "sn", "signature"}
	if len(vals) != len(required) {
		return "malformed_fields"
	}
	for _, k := range required {
		if len(vals[k]) != 1 || strings.TrimSpace(vals[k][0]) == "" {
			return "malformed_fields"
		}
	}
	did := vals.Get("did")
	if urlDID != "" && did != urlDID {
		return "did_mismatch"
	}
	fs := vals.Get("featureset")
	nfs := vals.Get("newfeatureset")
	pid := vals.Get("pid")
	sn := vals.Get("sn")
	sig := vals.Get("signature")

	if !decimalDigitsRe.MatchString(fs) || !hexDigitsRe.MatchString(nfs) ||
		!safeMetaValueRe.MatchString(pid) || !safeMetaValueRe.MatchString(sn) ||
		!safeMetaValueRe.MatchString(did) {
		return "unsupported_values"
	}
	if s.keys == nil {
		return "missing_key"
	}
	pub := s.keys.GetPublicKey(did)
	if pub == nil {
		return "missing_key"
	}
	canonical := fmt.Sprintf("did=%s&featureset=%s&newfeatureset=%s&pid=%s&sn=%s", did, fs, nfs, pid, sn)
	if err := VerifyDeviceInfoSignature(pub, canonical, sig); err != nil {
		return "invalid_signature"
	}
	if s.creds == nil {
		return "unmatched"
	}
	if _, exists := s.creds.ResolveDevice(did, did, ""); !exists {
		return "unmatched"
	}
	if _, err := s.creds.EnsureDevice(DeviceCredential{
		DID:           did,
		Model:         pid,
		SerialNumber:  sn,
		FeatureSet:    fs,
		NewFeatureSet: nfs,
	}); err != nil {
		return "write_failed"
	}
	return "stored"
}

func (s *HTTPServer) handleCreateCA(w http.ResponseWriter, trimmed string) {
	if s.turn == nil || !s.turn.Enabled() {
		writeJSON(w, http.StatusOK, wrapEnvelope(map[string]any{"ok": true, "route": trimmed}))
		return
	}
	payload := s.turn.Payload()
	if s.telemetry != nil {
		s.telemetry.Emit(TelemetryEvent{
			Kind:    "http",
			Summary: fmt.Sprintf("POST /fwapi/createca -> %s", payload.URL),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code":   200,
		"msg":    "success",
		"data":   payload,
		"result": payload,
	})
}

func (s *HTTPServer) handleAdminStatus(w http.ResponseWriter) {
	turnCfg := TURNConfig{Mode: TURNModeDisabled}
	turnRunning := false
	if s.turn != nil {
		turnCfg = s.turn.Config()
		turnRunning = s.turn.Running()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                       true,
		"api_url":                  s.apiURL(""),
		"mqtt_url":                 s.mqttURL(""),
		"advertised_https_port":    s.cfg.AdvertisedPort,
		"advertised_mqtt_tls_port": s.cfg.AdvertisedMQTTPort,
		"region":                   s.cfg.Region,
		"timezone":                 s.cfg.Timezone,
		"turn": map[string]any{
			"mode":    turnCfg.Mode,
			"host":    turnCfg.Host,
			"port":    turnCfg.Port,
			"running": turnRunning,
		},
	})
}

func (s *HTTPServer) handleAdminDevices(w http.ResponseWriter) {
	var devs []VacuumTelemetry
	if s.telemetry != nil {
		devs = s.telemetry.Snapshot(s.creds, s.keys)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"count":   len(devs),
		"devices": devs,
	})
}

func parseBodyParams(r *http.Request, rawBody []byte) url.Values {
	out := url.Values{}
	if len(rawBody) == 0 {
		return out
	}
	ct := strings.ToLower(strings.TrimSpace(strings.Split(r.Header.Get("Content-Type"), ";")[0]))
	if ct == "multipart/form-data" {
		if err := r.ParseMultipartForm(maxHTTPBodyBytes); err == nil && r.MultipartForm != nil {
			for k, vals := range r.MultipartForm.Value {
				out[k] = append(out[k], vals...)
			}
		}
		return out
	}
	bodyStr := strings.TrimSpace(string(rawBody))
	if strings.Contains(ct, "json") || strings.HasPrefix(bodyStr, "{") {
		var obj map[string]any
		if err := json.Unmarshal(rawBody, &obj); err == nil {
			for k, v := range obj {
				if v != nil {
					out.Add(k, fmt.Sprint(v))
				}
			}
		}
		return out
	}
	if parsed, err := url.ParseQuery(string(rawBody)); err == nil {
		return parsed
	}
	return out
}

func firstParam(q, b url.Values, keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(q.Get(k)); v != "" {
			return v
		}
		if v := strings.TrimSpace(b.Get(k)); v != "" {
			return v
		}
	}
	return ""
}

func pickHeader(h http.Header, keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(h.Get(k)); v != "" {
			return v
		}
	}
	return ""
}

func splitHostPort(raw string) (string, int) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", 0
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", 0
	}
	p := 0
	if ps := u.Port(); ps != "" {
		_, _ = fmt.Sscanf(ps, "%d", &p)
	}
	return u.Hostname(), p
}

func isAppAPIPath(trimmed string) bool {
	return strings.HasPrefix(trimmed, "/api/v") ||
		strings.HasPrefix(trimmed, "/user/") ||
		strings.HasPrefix(trimmed, "/v2/user/")
}

func defaultTimezoneForRegion(region string) string {
	switch strings.ToLower(strings.TrimSpace(region)) {
	case "eu":
		return "Europe/Berlin"
	case "cn":
		return "Asia/Shanghai"
	case "ru":
		return "Europe/Moscow"
	default:
		return "America/New_York"
	}
}

func wrapEnvelope(data any) map[string]any {
	return map[string]any{
		"success": true,
		"code":    200,
		"msg":     "success",
		"data":    data,
		"result":  data,
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
