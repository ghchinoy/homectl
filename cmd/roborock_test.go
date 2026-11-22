package cmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ghchinoy/homectl/modules/core"
	"github.com/ghchinoy/homectl/modules/roborock"
)

func TestRoborockControlMuxEndpoints(t *testing.T) {
	storage := core.NewDirStorage(t.TempDir())
	stack, err := roborock.NewStack(roborock.StackConfig{
		Storage: storage,
		Logger:  core.NewNoOpLogger(),
		TURN: roborock.TURNConfig{
			Mode: roborock.TURNModeExternal,
			Host: "r.example.com",
			Port: 3478,
		},
	})
	if err != nil {
		t.Fatalf("NewStack: %v", err)
	}
	_, err = stack.Creds.EnsureDevice(roborock.DeviceCredential{
		DID:             "144001",
		DUID:            "duid-144",
		Name:            "Saros 10R",
		Model:           "roborock.vacuum.a144",
		FirmwareVersion: "02.52.86",
	})
	if err != nil {
		t.Fatal(err)
	}

	mux := buildRoborockControlMux(stack, "r.example.com/", "r.example.com/")

	// 1. GET /api/roborock/devices
	req := httptest.NewRequest(http.MethodGet, "/api/roborock/devices", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/roborock/devices status: got: %d, want: 200", rec.Code)
	}
	var devResp struct {
		Count   int                        `json:"count"`
		Devices []roborock.VacuumTelemetry `json:"devices"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &devResp); err != nil {
		t.Fatal(err)
	}
	if devResp.Count != 1 || devResp.Devices[0].Model != "roborock.vacuum.a144" {
		t.Fatalf("devices response: got: %+v", devResp)
	}

	// 2. GET /api/roborock/camera
	req = httptest.NewRequest(http.MethodGet, "/api/roborock/camera?id=a144", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/roborock/camera status: got: %d, want: 200", rec.Code)
	}
	var camResp struct {
		Count   int                   `json:"count"`
		Cameras []roborock.CameraInfo `json:"cameras"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &camResp); err != nil {
		t.Fatal(err)
	}
	if camResp.Count != 1 || camResp.Cameras[0].BootstrapSource != roborock.CameraBootstrapCountryDomain {
		t.Fatalf("camera response: got: %+v", camResp)
	}

	// 3. POST /api/roborock/control dry-run
	body, _ := json.Marshal(map[string]any{"id": "a144", "action": "dock", "dry_run": true})
	req = httptest.NewRequest(http.MethodPost, "/api/roborock/control", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST /api/roborock/control dry_run status: got: %d, want: 200 (%s)", rec.Code, rec.Body.String())
	}
}
