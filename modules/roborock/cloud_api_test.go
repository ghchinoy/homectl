package roborock

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ghchinoy/homectl/modules/core"
)

func TestSignHawkHeaderDeterministic(t *testing.T) {
	rriot := CloudRRiot{
		U: "user123",
		S: "secret456",
		H: "hmac-key-789",
		K: "k-val",
	}
	got := SignHawkHeader(rriot, "/v3/user/homes/42", nil, nil, nil, 1700000000, "abcdef")
	if !strings.HasPrefix(got, `Hawk id="user123",s="secret456",ts="1700000000",nonce="abcdef",mac="`) {
		t.Fatalf("SignHawkHeader unexpected format: got: %s", got)
	}
	// Repeat call with identical inputs must produce identical MAC.
	got2 := SignHawkHeader(rriot, "/v3/user/homes/42", nil, nil, nil, 1700000000, "abcdef")
	if got != got2 {
		t.Fatalf("SignHawkHeader not deterministic: got: %s, want: %s", got2, got)
	}
}

func TestCloudClientV4LoginAndHomeData(t *testing.T) {
	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/getUrlByEmail":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 200,
				"data": map[string]any{
					"url":         srvURL,
					"country":     "US",
					"countrycode": "1",
				},
			})
		case "/api/v4/email/code/send":
			if err := r.ParseForm(); err != nil || r.FormValue("email") != "user@example.com" {
				t.Errorf("unexpected code/send form: %v", r.Form)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "msg": "success"})
		case "/api/v3/key/sign":
			s := r.URL.Query().Get("s")
			if len(s) != 16 {
				t.Errorf("key/sign s len: got: %d, want: 16", len(s))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 200,
				"data": map[string]any{"k": "signed-k-value"},
			})
		case "/api/v3/app/agreement/latest":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 200,
				"data": map[string]any{"majorVersion": 14, "minorVersion": 2},
			})
		case "/api/v4/auth/email/login/code":
			if r.Header.Get("x-mercy-k") != "signed-k-value" {
				t.Errorf("missing x-mercy-k header: got: %q", r.Header.Get("x-mercy-k"))
			}
			if err := r.ParseForm(); err != nil || r.FormValue("code") != "123456" {
				t.Errorf("unexpected login form: %v", r.Form)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 200,
				"data": map[string]any{
					"uid":         1001,
					"token":       "rr-token-abc",
					"rruid":       "rr123",
					"region":      "us",
					"country":     "US",
					"countrycode": "1",
					"rriot": map[string]any{
						"u": "u-id",
						"s": "s-val",
						"h": "h-secret",
						"k": "k-key",
						"r": map[string]any{"a": srvURL},
					},
				},
			})
		case "/api/v1/getHomeDetail":
			if r.Header.Get("Authorization") != "rr-token-abc" {
				t.Errorf("getHomeDetail auth: got: %q, want: rr-token-abc", r.Header.Get("Authorization"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 200,
				"data": map[string]any{"rrHomeId": 99001},
			})
		case "/v3/user/homes/99001":
			if !strings.HasPrefix(r.Header.Get("Authorization"), "Hawk ") {
				t.Errorf("homes auth header: got: %q, want Hawk prefix", r.Header.Get("Authorization"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result": map[string]any{
					"id":   99001,
					"name": "Test Home",
					"products": []map[string]any{
						{"id": "p1", "name": "Saros 10R", "model": "roborock.vacuum.a144"},
						{"id": "p2", "name": "Q5 Max+", "model": "roborock.vacuum.a125"},
					},
					"devices": []map[string]any{
						{
							"duid":      "duid-10r",
							"name":      "Upstairs Vacuum",
							"localKey":  "0123456789abcdef",
							"productId": "p1",
							"fv":        "02.18.04",
							"pv":        "1.0",
						},
						{
							"duid":      "duid-q5",
							"name":      "Basement Vacuum",
							"localKey":  "fedcba9876543210",
							"productId": "p2",
							"fv":        "02.11.00",
							"pv":        "1.0",
						},
					},
					"rooms": []map[string]any{
						{"id": 100, "name": "Kitchen"},
						{"id": 101, "name": "Living Room"},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	srvURL = srv.URL

	client := NewCloudClient("user@example.com", srv.URL)
	client.HTTPClient = srv.Client()

	if err := client.RequestEmailCode(t.Context()); err != nil {
		t.Fatalf("RequestEmailCode: %v", err)
	}
	ud, err := client.LoginWithCode(t.Context(), "123456")
	if err != nil {
		t.Fatalf("LoginWithCode: %v", err)
	}
	hd, err := client.GetHomeData(t.Context(), ud)
	if err != nil {
		t.Fatalf("GetHomeData: %v", err)
	}
	if len(hd.Devices) != 2 {
		t.Fatalf("devices count: got: %d, want: 2", len(hd.Devices))
	}

	storage := core.NewMemoryStorage()
	store, err := NewCloudDeviceStore(storage)
	if err != nil {
		t.Fatalf("NewCloudDeviceStore: %v", err)
	}
	saved, err := store.SaveHomeData("user@example.com", "us", hd)
	if err != nil {
		t.Fatalf("SaveHomeData: %v", err)
	}
	if len(saved) != 2 {
		t.Fatalf("saved count: got: %d, want: 2", len(saved))
	}

	// Update discovered LAN IPs and verify reload preserves everything.
	updated, err := store.UpdateDiscovered([]BroadcastDevice{
		{DUID: "duid-10r", IP: "192.0.2.50", Version: VersionL01},
	})
	if err != nil || updated != 1 {
		t.Fatalf("UpdateDiscovered: got: (%d, %v), want: (1, nil)", updated, err)
	}

	reloaded, err := NewCloudDeviceStore(storage)
	if err != nil {
		t.Fatalf("reload CloudDeviceStore: %v", err)
	}
	rec, ok := reloaded.Find("upstairs")
	if !ok {
		t.Fatal("Find('upstairs') = false, want true")
	}
	if rec.IP != "192.0.2.50" || rec.LocalProtocol != VersionL01 || rec.Model != "roborock.vacuum.a144" {
		t.Fatalf("reloaded device mismatch: got: %+v", rec)
	}
	if rooms := reloaded.Rooms(); rooms[100] != "Kitchen" {
		t.Fatalf("room 100: got: %q, want: Kitchen", rooms[100])
	}
}
