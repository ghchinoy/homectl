package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/ghchinoy/homectl/modules/core"
	"github.com/ghchinoy/homectl/modules/sonos"
	"github.com/ghchinoy/homectl/pkg/config"
	"github.com/ghchinoy/homectl/pkg/leap"
)

type mockLutronAPI struct {
	devices   []leap.Device
	statuses  []leap.ZoneStatus
	lastHref  string
	lastLevel float64
	allLevel  float64
}

func (m *mockLutronAPI) GetDevices() ([]leap.Device, error) {
	return m.devices, nil
}

func (m *mockLutronAPI) GetAllZoneStatuses() ([]leap.ZoneStatus, error) {
	return m.statuses, nil
}

func (m *mockLutronAPI) SetLevel(zoneHref string, level float64) error {
	m.lastHref = zoneHref
	m.lastLevel = level
	return nil
}

func (m *mockLutronAPI) SetAllLevels(level float64) error {
	m.allLevel = level
	return nil
}

func TestAPIMux_LutronAndSecurityGuards(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	_ = config.EnsureDir()
	_ = config.SaveNicknames(map[string]string{
		"/zone/1":      "Living Room Dimmer",
		"192.168.1.50": "Sonos Bridge",
	})

	memStore := core.NewMemoryStorage()
	sonos.SetDefaultStorage(memStore)
	_ = sonos.SaveCache([]sonos.Device{
		{
			Name:        "Bridge",
			IP:          "192.168.1.50",
			RinconID:    "RINCON_BRIDGE",
			ModelName:   "Sonos Bridge",
			ModelNumber: "ZB100",
			Generation:  sonos.GenerationS1,
			IsRenderer:  false,
		},
	})

	mockLutron := &mockLutronAPI{
		devices: []leap.Device{
			{
				Href:       "/device/1",
				Name:       "Dimmer 1",
				DeviceType: "WallDimmer",
				LocalZones: []leap.ZoneLink{{Href: "/zone/1"}},
			},
		},
		statuses: []leap.ZoneStatus{
			{Href: "/zone/1/status", Level: 65},
		},
	}

	mux := newAPIMux(mockLutron, t.TempDir(), false)

	t.Run("lutron_devices_enriches_nicknames", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/lutron/devices", nil)
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /api/lutron/devices status = %d, want %d", rec.Code, http.StatusOK)
		}
		var got []leap.Device
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("json.Unmarshal(devices) failed: %v", err)
		}
		if len(got) != 1 || got[0].Nickname != "Living Room Dimmer" {
			t.Errorf("GET /api/lutron/devices = %+v, want Nickname=Living Room Dimmer", got)
		}
	})

	t.Run("debug_endpoint_disabled_by_default", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/debug/lutron", nil)
		mux.ServeHTTP(rec, req)
		// Falls through to FileServer returning 404 when --debug is false
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET /api/debug/lutron (debug=false) status = %d, want %d", rec.Code, http.StatusNotFound)
		}

		debugMux := newAPIMux(mockLutron, t.TempDir(), true)
		debugRec := httptest.NewRecorder()
		debugMux.ServeHTTP(debugRec, req)
		if debugRec.Code != http.StatusOK {
			t.Errorf("GET /api/debug/lutron (debug=true) status = %d, want %d", debugRec.Code, http.StatusOK)
		}
	})

	t.Run("cross_origin_post_blocked_across_all_mutating_routes", func(t *testing.T) {
		mutatingRoutes := []string{
			"/api/lutron/set",
			"/api/lutron/all",
			"/api/sonos/control",
			"/api/sonos/play-favorite",
			"/api/sonos/play-stream",
			"/api/sonos/queue-add",
			"/api/cast/control",
		}
		for _, route := range mutatingRoutes {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "http://192.168.1.10:8080"+route, strings.NewReader(`{}`))
			req.Host = "192.168.1.10:8080"
			req.Header.Set("Origin", "https://attacker.example.com")
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Errorf("POST %s (cross-origin) status = %d, want %d", route, rec.Code, http.StatusForbidden)
			}
		}
	})

	t.Run("same_origin_lutron_mutations_succeed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "http://192.168.1.10:8080/api/lutron/set", strings.NewReader(`{"href":"/zone/1","level":80}`))
		req.Host = "192.168.1.10:8080"
		req.Header.Set("Origin", "http://192.168.1.10:8080")
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("POST /api/lutron/set status = %d, want %d", rec.Code, http.StatusOK)
		}
		if mockLutron.lastHref != "/zone/1" || mockLutron.lastLevel != 80 {
			t.Errorf("SetLevel(/zone/1, 80) = (%q, %.0f), want (/zone/1, 80)", mockLutron.lastHref, mockLutron.lastLevel)
		}
	})

	t.Run("oversized_post_body_rejected_with_413", func(t *testing.T) {
		huge := `{"href":"/zone/1","pad":"` + strings.Repeat("x", maxAPIJSONBodyBytes+2048) + `"}`
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/lutron/set", strings.NewReader(huge))
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("POST /api/lutron/set (oversized) status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
		}
	})

	t.Run("invalid_device_ips_rejected_with_400", func(t *testing.T) {
		getRoutes := []string{
			"/api/sonos/favorites?ip=127.0.0.1",
			"/api/sonos/services?ip=speaker.local",
			"/api/sonos/queue?ip=169.254.169.254",
			"/api/cast/status?ip=192.168.1.50:8009",
			"/api/security/stream?ip=127.0.0.1",
		}
		for _, target := range getRoutes {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, target, nil)
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("GET %s status = %d, want %d", target, rec.Code, http.StatusBadRequest)
			}
		}

		postCases := []struct {
			route string
			body  string
		}{
			{route: "/api/sonos/control", body: `{"ip":"127.0.0.1","action":"play"}`},
			{route: "/api/sonos/play-favorite", body: `{"ip":"speaker.local","favorite_id":"FV:2/1"}`},
			{route: "/api/sonos/play-stream", body: `{"ip":"169.254.169.254","url":"https://example.com/a.mp3"}`},
			{route: "/api/sonos/queue-add", body: `{"ip":"0.0.0.0","uri":"x-file-cifs://nas/a.mp3"}`},
			{route: "/api/cast/control", body: `{"ip":"localhost","action":"pause"}`},
		}
		for _, pc := range postCases {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, pc.route, strings.NewReader(pc.body))
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("POST %s (%s) status = %d, want %d", pc.route, pc.body, rec.Code, http.StatusBadRequest)
			}
		}
	})

	t.Run("sonos_status_skips_non_renderer_bridge", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/sonos/status", nil)
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /api/sonos/status status = %d, want %d", rec.Code, http.StatusOK)
		}
		var got map[string]map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatalf("json.Unmarshal(sonos/status) failed: %v", err)
		}
		bridgeEntry, ok := got["192.168.1.50"]
		if !ok || bridgeEntry["status"] != "BRIDGE" || bridgeEntry["is_renderer"] != false {
			t.Errorf("GET /api/sonos/status[192.168.1.50] = %+v, want status=BRIDGE is_renderer=false", bridgeEntry)
		}
	})
}

func TestResolveUIHandler_EmbeddedFallbackAndDiskPrecedence(t *testing.T) {
	embedded := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<!doctype html><title>embedded-homectl</title>")},
	}

	t.Run("falls back to embedded FS when default uiDir lacks index.html", func(t *testing.T) {
		missingDir := filepath.Join(t.TempDir(), "nonexistent-ui")
		handler, source := resolveUIHandler(missingDir, false, embedded)
		if source != "embedded://ui/dist" {
			t.Fatalf("resolveUIHandler(missing, flagChanged=false) source = %q, want %q", source, "embedded://ui/dist")
		}
		mux := newAPIMuxWithUI(&mockLutronAPI{}, handler, false)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "embedded-homectl") {
			t.Errorf("GET / (embedded) = (%d, %q), want (200, contains embedded-homectl)", rec.Code, rec.Body.String())
		}
	})

	t.Run("serves disk directory when index.html is present on disk", func(t *testing.T) {
		diskDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(diskDir, "index.html"), []byte("<!doctype html><title>disk-homectl</title>"), 0644); err != nil {
			t.Fatalf("os.WriteFile(index.html) failed: %v", err)
		}
		handler, source := resolveUIHandler(diskDir, false, embedded)
		if source == "embedded://ui/dist" {
			t.Fatalf("resolveUIHandler(diskDir, flagChanged=false) source = %q, want disk path", source)
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "disk-homectl") {
			t.Errorf("GET / (disk) = (%d, %q), want (200, contains disk-homectl)", rec.Code, rec.Body.String())
		}
	})

	t.Run("honors explicit --ui flag even when directory is empty", func(t *testing.T) {
		explicitDir := t.TempDir()
		_, source := resolveUIHandler(explicitDir, true, embedded)
		if source == "embedded://ui/dist" {
			t.Errorf("resolveUIHandler(explicitDir, flagChanged=true) source = %q, want explicit disk path", source)
		}
	})
}
