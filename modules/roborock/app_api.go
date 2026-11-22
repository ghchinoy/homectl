package roborock

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AppUserSession represents an authenticated Home Assistant / LocalRock / Roborock app session.
type AppUserSession struct {
	UID          int64  `json:"uid"`
	Token        string `json:"token"`
	RRUID        string `json:"rruid"`
	HawkID       string `json:"hawk_id"`       // rriot.u
	HawkSession  string `json:"hawk_session"`  // rriot.s
	HawkKey      string `json:"hawk_key"`      // rriot.h
	LocalKey     string `json:"local_key"`     // rriot.k
	MQTTUsername string `json:"mqtt_username"` // md5hex(u + ":" + k)[2:10]
	MQTTPassword string `json:"mqtt_password"` // md5hex(s + ":" + k)[16:]
}

// AppAPI provides optional Home Assistant (`python-roborock` Region: Manual) and LocalRock
// REST compatibility (Hawk MAC verification, email+PIN login, home/device inventory).
type AppAPI struct {
	mu           sync.RWMutex
	email        string
	pin          string
	apiURL       string
	mqttURL      string
	region       string
	creds        *CredentialStore
	telemetry    *TelemetryHub
	keys         *KeyStore
	sessions     map[string]AppUserSession // keyed by HawkID (rriot.u)
	homeID       int64
}

// NewAppAPI creates a Home Assistant / LocalRock compatible REST handler attached to a Stack.
func NewAppAPI(email, pin, apiURL, mqttURL, region string, creds *CredentialStore, telemetry *TelemetryHub, keys *KeyStore) *AppAPI {
	if region == "" {
		region = "us"
	}
	return &AppAPI{
		email:     strings.ToLower(strings.TrimSpace(email)),
		pin:       strings.TrimSpace(pin),
		apiURL:    apiURL,
		mqttURL:   mqttURL,
		region:    strings.ToUpper(region),
		creds:     creds,
		telemetry: telemetry,
		keys:      keys,
		sessions:  make(map[string]AppUserSession),
		homeID:    100001,
	}
}

// IssueSession creates a new persisted app/HA session and computes its derived MQTT credentials.
func (a *AppAPI) IssueSession() AppUserSession {
	u := "u_" + randomHex(6)
	s := "s_" + randomHex(8)
	h := randomHex(5)
	k := randomHex(8)
	sess := AppUserSession{
		UID:          100001,
		Token:        "tok_" + randomHex(12),
		RRUID:        "rruid_" + randomHex(6),
		HawkID:       u,
		HawkSession:  s,
		HawkKey:      h,
		LocalKey:     k,
		MQTTUsername: md5Hex(u + ":" + k)[2:10],
		MQTTPassword: md5Hex(s + ":" + k)[16:],
	}
	a.mu.Lock()
	a.sessions[u] = sess
	a.mu.Unlock()
	return sess
}

// VerifyMQTTUser checks whether (username, password) matches any issued HA/LocalRock session.
func (a *AppAPI) VerifyMQTTUser(username, password string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	for _, s := range a.sessions {
		if subtle.ConstantTimeCompare([]byte(s.MQTTUsername), []byte(username)) == 1 &&
			subtle.ConstantTimeCompare([]byte(s.MQTTPassword), []byte(password)) == 1 {
			return true
		}
	}
	return false
}

// BuildHawkAuthorization builds a Roborock Hawk Authorization header (matching python-roborock & local_roborock_server).
func BuildHawkAuthorization(sess AppUserSession, path string, query url.Values, form url.Values, jsonBody []byte, ts int64, nonce string) string {
	mac := computeHawkMAC(sess.HawkID, sess.HawkSession, sess.HawkKey, path, query, form, jsonBody, ts, nonce)
	return fmt.Sprintf(`Hawk id="%s",s="%s",ts="%d",nonce="%s",mac="%s"`, sess.HawkID, sess.HawkSession, ts, nonce, mac)
}

// VerifyHawkHeader verifies an incoming Authorization: Hawk ... header against active sessions.
func (a *AppAPI) VerifyHawkHeader(authHeader, path string, query url.Values, form url.Values, jsonBody []byte) (AppUserSession, bool) {
	attrs := parseHawkHeader(authHeader)
	if attrs == nil {
		return AppUserSession{}, false
	}
	a.mu.RLock()
	sess, ok := a.sessions[attrs["id"]]
	a.mu.RUnlock()
	if !ok || subtle.ConstantTimeCompare([]byte(sess.HawkSession), []byte(attrs["s"])) != 1 {
		return AppUserSession{}, false
	}
	ts, err := strconv.ParseInt(attrs["ts"], 10, 64)
	if err != nil {
		return AppUserSession{}, false
	}
	if diff := time.Now().Unix() - ts; diff > 300 || diff < -300 {
		return AppUserSession{}, false
	}
	wantMAC := computeHawkMAC(sess.HawkID, sess.HawkSession, sess.HawkKey, path, query, form, jsonBody, ts, attrs["nonce"])
	if subtle.ConstantTimeCompare([]byte(wantMAC), []byte(attrs["mac"])) != 1 {
		return AppUserSession{}, false
	}
	return sess, true
}

// ServeHTTP dispatches Home Assistant / LocalRock REST API endpoints.
func (a *AppAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cleanPath := strings.TrimRight(StripRoborockPrefix(r.URL.Path), "/")

	switch {
	case cleanPath == "/api/v1/getUrlByEmail":
		writeJSON(w, http.StatusOK, map[string]any{
			"code": 200,
			"msg":  "success",
			"data": map[string]any{
				"url":         a.apiURL,
				"country":     a.region,
				"countrycode": "1",
			},
		})
		return

	case strings.Contains(cleanPath, "/sendEmailCode") || strings.Contains(cleanPath, "/sendSmsCode"):
		writeJSON(w, http.StatusOK, map[string]any{"code": 200, "msg": "success", "data": nil})
		return

	case strings.Contains(cleanPath, "/loginWithCode") || strings.Contains(cleanPath, "/login"):
		_ = r.ParseForm()
		email := strings.ToLower(strings.TrimSpace(firstParam(r.URL.Query(), r.Form, "username", "email")))
		code := strings.TrimSpace(firstParam(r.URL.Query(), r.Form, "code", "password"))
		if a.email != "" && email != "" && email != a.email {
			writeJSON(w, http.StatusOK, map[string]any{"code": 2018, "msg": "invalid email", "data": nil})
			return
		}
		if a.pin != "" && code != a.pin {
			writeJSON(w, http.StatusOK, map[string]any{"code": 2018, "msg": "invalid login pin", "data": nil})
			return
		}
		sess := a.IssueSession()
		writeJSON(w, http.StatusOK, map[string]any{
			"code": 200,
			"msg":  "success",
			"data": map[string]any{
				"uid":   sess.UID,
				"token": sess.Token,
				"rruid": sess.RRUID,
				"rriot": map[string]any{
					"u": sess.HawkID,
					"s": sess.HawkSession,
					"h": sess.HawkKey,
					"k": sess.LocalKey,
					"r": map[string]any{
						"r": a.region,
						"a": a.apiURL,
						"m": a.mqttURL,
						"l": a.apiURL,
					},
				},
			},
		})
		return

	case cleanPath == "/api/v1/getHomeDetail":
		writeJSON(w, http.StatusOK, map[string]any{
			"code": 200,
			"msg":  "success",
			"data": map[string]any{
				"rrHomeId": a.homeID,
				"name":     "Home",
			},
		})
		return

	case strings.HasPrefix(cleanPath, "/user/homes/"):
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"result":  a.buildHomeData(),
		})
		return
	}

	writeJSON(w, http.StatusOK, wrapEnvelope(map[string]any{"ok": true, "route": cleanPath}))
}

func (a *AppAPI) buildHomeData() map[string]any {
	var devices []map[string]any
	productsByID := make(map[string]map[string]any)
	now := time.Now().Unix()

	onlineByDID := make(map[string]bool)
	if a.telemetry != nil {
		for _, snap := range a.telemetry.Snapshot(a.creds, a.keys) {
			onlineByDID[snap.DID] = snap.Connected
		}
	}

	if a.creds != nil {
		for _, d := range a.creds.ListDevices() {
			duid := d.DUID
			if duid == "" {
				duid = d.DID
			}
			pid := d.ProductID
			if pid == "" {
				pid = "prod_" + duid
			}
			model := d.Model
			if model == "" {
				model = "roborock.vacuum.a144"
			}
			fv := d.FirmwareVersion
			if fv == "" {
				fv = "02.52.86"
			}
			devices = append(devices, map[string]any{
				"duid":       duid,
				"name":       d.Name,
				"localKey":   d.LocalKey,
				"productId":  pid,
				"fv":         fv,
				"pv":         "1.0",
				"activeTime": now,
				"online":     onlineByDID[d.DID],
				"sn":         d.SerialNumber,
			})
			if _, exists := productsByID[pid]; !exists {
				productsByID[pid] = map[string]any{
					"id":       pid,
					"name":     DefaultProductName(model),
					"model":    model,
					"category": "robot.vacuum.cleaner",
				}
			}
		}
	}
	products := make([]map[string]any, 0, len(productsByID))
	for _, p := range productsByID {
		products = append(products, p)
	}
	return map[string]any{
		"id":              a.homeID,
		"name":            "Home",
		"products":        products,
		"devices":         devices,
		"receivedDevices": []any{},
		"rooms":           []any{},
	}
}

func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

func md5HexBytes(b []byte) string {
	sum := md5.Sum(b)
	return hex.EncodeToString(sum[:])
}

func hashSortedValues(vals url.Values) string {
	if len(vals) == 0 {
		return ""
	}
	keys := make([]string, 0, len(vals))
	for k := range vals {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, vals.Get(k)))
	}
	return md5Hex(strings.Join(parts, "&"))
}

func computeHawkMAC(hawkID, hawkSession, hawkKey, path string, query url.Values, form url.Values, jsonBody []byte, ts int64, nonce string) string {
	qHash := hashSortedValues(query)
	var bHash string
	if len(jsonBody) > 0 {
		bHash = md5HexBytes(jsonBody)
	} else {
		bHash = hashSortedValues(form)
	}
	pre := strings.Join([]string{
		hawkID,
		hawkSession,
		nonce,
		strconv.FormatInt(ts, 10),
		md5Hex(path),
		qHash,
		bHash,
	}, ":")
	mac := hmac.New(sha256.New, []byte(hawkKey))
	mac.Write([]byte(pre))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func parseHawkHeader(raw string) map[string]string {
	s := strings.TrimSpace(raw)
	if len(s) < 5 || !strings.EqualFold(s[:5], "hawk ") {
		return nil
	}
	out := make(map[string]string)
	for _, item := range strings.Split(s[5:], ",") {
		k, v, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		k = strings.ToLower(strings.TrimSpace(k))
		v = strings.Trim(strings.TrimSpace(v), `"`)
		out[k] = v
	}
	for _, req := range []string{"id", "s", "ts", "nonce", "mac"} {
		if out[req] == "" {
			return nil
		}
	}
	return out
}
