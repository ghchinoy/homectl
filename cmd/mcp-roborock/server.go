package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ghchinoy/homectl/modules/core"
	"github.com/ghchinoy/homectl/modules/roborock"
	"github.com/ghchinoy/homectl/pkg/config"
	"github.com/ghchinoy/homectl/pkg/version"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const defaultControlURL = "http://127.0.0.1:8889"

// ServerConfig configures the Roborock MCP server.
type ServerConfig struct {
	ControlURL string
	HTTPClient *http.Client
	Storage    core.Storage
}

// ServerOption mutates ServerConfig.
type ServerOption func(*ServerConfig)

// WithControlURL overrides the Roborock local control API base URL.
func WithControlURL(u string) ServerOption {
	return func(c *ServerConfig) {
		if strings.TrimSpace(u) != "" {
			c.ControlURL = strings.TrimRight(strings.TrimSpace(u), "/")
		}
	}
}

// WithHTTPClient overrides the HTTP client (useful for httptest unit tests).
func WithHTTPClient(cl *http.Client) ServerOption {
	return func(c *ServerConfig) {
		if cl != nil {
			c.HTTPClient = cl
		}
	}
}

// WithStorage overrides the fallback XDG storage directory.
func WithStorage(s core.Storage) ServerOption {
	return func(c *ServerConfig) {
		if s != nil {
			c.Storage = s
		}
	}
}

func defaultServerConfig() ServerConfig {
	ctrlURL := strings.TrimSpace(os.Getenv("HOMECTL_ROBOROCK_CONTROL_URL"))
	if ctrlURL == "" {
		if cfg := config.LoadConfig(); cfg.Roborock != nil && strings.TrimSpace(cfg.Roborock.ControlAPIURL) != "" {
			ctrlURL = strings.TrimSpace(cfg.Roborock.ControlAPIURL)
		}
	}
	if ctrlURL == "" {
		ctrlURL = defaultControlURL
	}
	return ServerConfig{
		ControlURL: strings.TrimRight(ctrlURL, "/"),
		HTTPClient: &http.Client{Timeout: 12 * time.Second},
		Storage:    core.NewDirStorage(filepath.Join(config.ConfigDir(), "roborock")),
	}
}

// --- Parameter & SEP-2106 Result Structs ---

type ListDevicesParams struct{}

type ListDevicesResult struct {
	Count   int                        `json:"count"`
	Devices []roborock.VacuumTelemetry `json:"devices"`
}

type DeviceIDParams struct {
	ID string `json:"id" jsonschema:"Vacuum DID, DUID, friendly name, or model code such as a125, a144, or a288 (required)"`
}

type StatusResult struct {
	ID          string          `json:"id"`
	Status      json.RawMessage `json:"status,omitempty"`
	Consumables json.RawMessage `json:"consumables,omitempty"`
}

type RoomsResult struct {
	ID    string          `json:"id"`
	Rooms json.RawMessage `json:"rooms,omitempty"`
}

type TelemetryParams struct {
	ID    string `json:"id,omitempty" jsonschema:"Optional vacuum DID/DUID filter"`
	Limit int    `json:"limit,omitempty" jsonschema:"Maximum number of recent events to return (default 50, max 500)"`
}

type TelemetryResult struct {
	Count  int                       `json:"count"`
	Events []roborock.TelemetryEvent `json:"events"`
}

type ControlParams struct {
	ID        string `json:"id" jsonschema:"Vacuum DID, DUID, friendly name, or model code (required)"`
	Action    string `json:"action" jsonschema:"Vacuum action: 'start', 'pause', 'stop', 'dock', 'find', 'clean_rooms', 'fan_power', 'water_mode', 'request_map' (required)"`
	Rooms     []int  `json:"rooms,omitempty" jsonschema:"Segment/room IDs for 'clean_rooms'"`
	Repeat    int    `json:"repeat,omitempty" jsonschema:"Cleaning passes for 'clean_rooms' (default 1)"`
	FanPower  int    `json:"fan_power,omitempty" jsonschema:"Suction code for 'fan_power' (e.g. 101 Quiet, 102 Balanced, 103 Turbo, 104 Max)"`
	WaterMode int    `json:"water_mode,omitempty" jsonschema:"Mop water intensity for 'water_mode' (200 Off, 201 Mild, 202 Moderate, 203 Intense)"`
	DryRun    bool   `json:"dry_run,omitempty" jsonschema:"Simulate the command without mutating vacuum state"`
}

type ControlResult struct {
	OK     bool            `json:"ok"`
	DryRun bool            `json:"dry_run,omitempty"`
	ID     string          `json:"id"`
	Action string          `json:"action"`
	Result json.RawMessage `json:"result,omitempty"`
}

type CameraInfoParams struct {
	ID string `json:"id,omitempty" jsonschema:"Optional vacuum DID/DUID/name/model filter"`
}

type CameraInfoResult struct {
	Count   int                   `json:"count"`
	Cameras []roborock.CameraInfo `json:"cameras"`
}

func getJSON(ctx context.Context, cl *http.Client, rawURL string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	resp, err := cl.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.Unmarshal(body, dst)
}

func postJSON(ctx context.Context, cl *http.Client, rawURL string, payload, dst any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := cl.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return json.Unmarshal(body, dst)
}

// CreateMCPServer builds the Roborock MCP stdio server with <=6 consolidated SEP-2106 tools.
func CreateMCPServer(opts ...ServerOption) *mcp.Server {
	cfg := defaultServerConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	s := mcp.NewServer(&mcp.Implementation{
		Name:    "homectl-roborock",
		Version: version.Version,
	}, nil)

	mcp.AddTool(s, &mcp.Tool{
		Name:        "roborock_list_devices",
		Description: "List all Roborock vacuums managed by the local server with online state, battery, cleaning state, firmware version, and RSA key-recovery progress.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ ListDevicesParams) (*mcp.CallToolResult, ListDevicesResult, error) {
		var out ListDevicesResult
		if err := getJSON(ctx, cfg.HTTPClient, cfg.ControlURL+"/api/roborock/devices", &out); err != nil {
			ks, _ := roborock.NewKeyStore(cfg.Storage, nil, nil)
			cs, _ := roborock.NewCredentialStore(cfg.Storage)
			out.Devices = roborock.NewTelemetryHub(10, "").Snapshot(cs, ks)
			out.Count = len(out.Devices)
		}
		if out.Devices == nil {
			out.Devices = []roborock.VacuumTelemetry{}
		}
		return nil, out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "roborock_get_status",
		Description: "Query live RPC status (battery, state, error code, fan power, dock/mop mode) and consumable wear times from a connected Roborock vacuum.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, p DeviceIDParams) (*mcp.CallToolResult, StatusResult, error) {
		if strings.TrimSpace(p.ID) == "" {
			return nil, StatusResult{}, fmt.Errorf("id is required")
		}
		var out StatusResult
		u := cfg.ControlURL + "/api/roborock/status?id=" + url.QueryEscape(p.ID)
		if err := getJSON(ctx, cfg.HTTPClient, u, &out); err != nil {
			return nil, StatusResult{}, err
		}
		return nil, out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "roborock_get_telemetry",
		Description: "Return recent decoded MQTT telemetry, RPC request/response traces, OTA firmware reports, and onboarding key-recovery events.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, p TelemetryParams) (*mcp.CallToolResult, TelemetryResult, error) {
		limit := p.Limit
		if limit <= 0 {
			limit = 50
		}
		if limit > 500 {
			limit = 500
		}
		u := fmt.Sprintf("%s/api/roborock/telemetry?limit=%d", cfg.ControlURL, limit)
		if strings.TrimSpace(p.ID) != "" {
			u += "&id=" + url.QueryEscape(p.ID)
		}
		var out TelemetryResult
		if err := getJSON(ctx, cfg.HTTPClient, u, &out); err != nil {
			return nil, TelemetryResult{}, err
		}
		if out.Events == nil {
			out.Events = []roborock.TelemetryEvent{}
		}
		return nil, out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "roborock_control",
		Description: "Control a Roborock vacuum (start, pause, stop, dock, find, clean_rooms, fan_power, water_mode, request_map). Supports dry_run simulation.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, p ControlParams) (*mcp.CallToolResult, ControlResult, error) {
		if strings.TrimSpace(p.ID) == "" || strings.TrimSpace(p.Action) == "" {
			return nil, ControlResult{}, fmt.Errorf("id and action are required")
		}
		var out ControlResult
		if err := postJSON(ctx, cfg.HTTPClient, cfg.ControlURL+"/api/roborock/control", p, &out); err != nil {
			return nil, ControlResult{}, err
		}
		return nil, out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "roborock_get_rooms",
		Description: "Fetch the segment/room ID mapping from a connected Roborock vacuum for targeted room cleaning.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, p DeviceIDParams) (*mcp.CallToolResult, RoomsResult, error) {
		if strings.TrimSpace(p.ID) == "" {
			return nil, RoomsResult{}, fmt.Errorf("id is required")
		}
		var out RoomsResult
		u := cfg.ControlURL + "/api/roborock/rooms?id=" + url.QueryEscape(p.ID)
		if err := getJSON(ctx, cfg.HTTPClient, u, &out); err != nil {
			return nil, RoomsResult{}, err
		}
		return nil, out, nil
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "roborock_camera_info",
		Description: "Inspect WebRTC TURN/STUN relay status and 15-character camera bootstrap diagnostics for camera-capable vacuums (e.g. Saros 10R a144, Saros 20 a288).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, p CameraInfoParams) (*mcp.CallToolResult, CameraInfoResult, error) {
		u := cfg.ControlURL + "/api/roborock/camera"
		if strings.TrimSpace(p.ID) != "" {
			u += "?id=" + url.QueryEscape(p.ID)
		}
		var out CameraInfoResult
		if err := getJSON(ctx, cfg.HTTPClient, u, &out); err != nil {
			return nil, CameraInfoResult{}, err
		}
		if out.Cameras == nil {
			out.Cameras = []roborock.CameraInfo{}
		}
		return nil, out, nil
	})

	return s
}
