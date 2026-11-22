package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ghchinoy/homectl/modules/core"
	"github.com/ghchinoy/homectl/modules/roborock"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRoborockMCPTools(t *testing.T) {
	mux := http.NewServeMux()
	bat := 92
	mux.HandleFunc("/api/roborock/devices", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(ListDevicesResult{
			Count: 1,
			Devices: []roborock.VacuumTelemetry{{
				DID:       "144001",
				DUID:      "duid-144",
				Name:      "Roborock Saros 10R",
				Model:     "roborock.vacuum.a144",
				Connected: true,
				Battery:   &bat,
				StateName: "charging",
			}},
		})
	})
	mux.HandleFunc("/api/roborock/status", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(StatusResult{
			ID:     r.URL.Query().Get("id"),
			Status: json.RawMessage(`[{"battery":92,"state":8}]`),
		})
	})
	mux.HandleFunc("/api/roborock/telemetry", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(TelemetryResult{
			Count: 1,
			Events: []roborock.TelemetryEvent{{
				Kind:    "mqtt_dps",
				DID:     "144001",
				Summary: "121=8 122=92",
			}},
		})
	})
	mux.HandleFunc("/api/roborock/control", func(w http.ResponseWriter, r *http.Request) {
		var req ControlParams
		_ = json.NewDecoder(r.Body).Decode(&req)
		_ = json.NewEncoder(w).Encode(ControlResult{
			OK:     true,
			DryRun: req.DryRun,
			ID:     req.ID,
			Action: req.Action,
		})
	})
	mux.HandleFunc("/api/roborock/camera", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(CameraInfoResult{
			Count: 1,
			Cameras: []roborock.CameraInfo{{
				DID:             "144001",
				Model:           "roborock.vacuum.a144",
				BootstrapSource: roborock.CameraBootstrapCountryDomain,
				TURNMode:        roborock.TURNModeEmbedded,
				TURNRunning:     true,
			}},
		})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	srv := CreateMCPServer(
		WithControlURL(ts.URL),
		WithHTTPClient(ts.Client()),
		WithStorage(core.NewMemoryStorage()),
	)

	t1, t2 := mcp.NewInMemoryTransports()
	ctx := t.Context()
	srvSession, err := srv.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer srvSession.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	cliSession, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cliSession.Close()

	toolsResp, err := cliSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(toolsResp.Tools) != 6 {
		t.Fatalf("tool count: got: %d, want: 6", len(toolsResp.Tools))
	}

	res, err := cliSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "roborock_list_devices",
		Arguments: map[string]any{},
	})
	if err != nil || res.IsError {
		t.Fatalf("roborock_list_devices failed: err=%v isError=%v", err, res.IsError)
	}

	ctrlRes, err := cliSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "roborock_control",
		Arguments: map[string]any{
			"id":      "a144",
			"action":  "dock",
			"dry_run": true,
		},
	})
	if err != nil || ctrlRes.IsError {
		t.Fatalf("roborock_control failed: err=%v isError=%v", err, ctrlRes.IsError)
	}
}
