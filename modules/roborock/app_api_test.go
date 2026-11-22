package roborock

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ghchinoy/homectl/modules/core"
)

func TestAppAPILoginHawkAuthAndHomeData(t *testing.T) {
	storage := core.NewMemoryStorage()
	cs, _ := NewCredentialStore(storage)
	ks, _ := NewKeyStore(storage, nil, nil)
	hub := NewTelemetryHub(16, "")
	_, _ = cs.EnsureDevice(DeviceCredential{
		DID:      "288001",
		DUID:     "duid-a288",
		Name:     "Saros 20",
		Model:    "roborock.vacuum.a288",
		LocalKey: "0123456789abcdef",
	})

	api := NewAppAPI("user@example.com", "123456", "https://api-r.example.com", "ssl://api-r.example.com:8881", "us", cs, hub, ks)

	// 1. Wrong PIN rejected
	form := url.Values{"username": {"user@example.com"}, "code": {"000000"}}
	req := httptest.NewRequest(http.MethodPost, "/api/v4/auth/email/login/code", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), `"code":2018`) {
		t.Fatalf("wrong PIN response: got: %s, want code 2018", rec.Body.String())
	}

	// 2. Valid PIN issues session + valid Hawk + valid MQTT user
	form.Set("code", "123456")
	req = httptest.NewRequest(http.MethodPost, "/api/v4/auth/email/login/code", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	var loginResp struct {
		Code int `json:"code"`
		Data struct {
			RRiot struct {
				U string `json:"u"`
				S string `json:"s"`
				H string `json:"h"`
				K string `json:"k"`
			} `json:"rriot"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &loginResp); err != nil || loginResp.Code != 200 {
		t.Fatalf("valid login failed: %v body=%s", err, rec.Body.String())
	}

	sess := AppUserSession{
		HawkID:      loginResp.Data.RRiot.U,
		HawkSession: loginResp.Data.RRiot.S,
		HawkKey:     loginResp.Data.RRiot.H,
		LocalKey:    loginResp.Data.RRiot.K,
	}
	authHdr := BuildHawkAuthorization(sess, "/user/homes/100001", nil, nil, nil, time.Now().Unix(), "abc123")
	if _, ok := api.VerifyHawkHeader(authHdr, "/user/homes/100001", nil, nil, nil); !ok {
		t.Fatalf("VerifyHawkHeader returned false for valid signature")
	}
	mqttUser := md5Hex(sess.HawkID + ":" + sess.LocalKey)[2:10]
	mqttPass := md5Hex(sess.HawkSession + ":" + sess.LocalKey)[16:]
	if !api.VerifyMQTTUser(mqttUser, mqttPass) {
		t.Fatalf("VerifyMQTTUser returned false for derived session credentials")
	}

	// 3. GET /user/homes/100001 returns Saros 20
	req = httptest.NewRequest(http.MethodGet, "/user/homes/100001", nil)
	rec = httptest.NewRecorder()
	api.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), `"duid":"duid-a288"`) {
		t.Fatalf("GET /user/homes/100001 missing Saros 20: %s", rec.Body.String())
	}
}
