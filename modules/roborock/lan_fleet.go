package roborock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// LANFleet manages direct LAN TCP (:58867) monitoring and control for vacuums whose
// local_key was fetched once via Roborock cloud login (cloud_devices.json), allowing
// all vacuums to remain paired to the official Roborock mobile app.
type LANFleet struct {
	Store       *CloudDeviceStore
	DiscoverFn  func(ctx context.Context, timeout time.Duration) ([]BroadcastDevice, error)
	DialTimeout time.Duration

	mu          sync.Mutex
	lastScanAt  time.Time
	minScanGap  time.Duration
	cachedSnaps map[string]VacuumTelemetry
}

// NewLANFleet constructs a LANFleet backed by store.
func NewLANFleet(store *CloudDeviceStore) *LANFleet {
	return &LANFleet{
		Store:       store,
		DiscoverFn:  DiscoverLAN,
		DialTimeout: 4 * time.Second,
		minScanGap:  30 * time.Second,
		cachedSnaps: make(map[string]VacuumTelemetry),
	}
}

func (f *LANFleet) discover(ctx context.Context, timeout time.Duration) ([]BroadcastDevice, error) {
	fn := f.DiscoverFn
	if fn == nil {
		fn = DiscoverLAN
	}
	return fn(ctx, timeout)
}

// SyncDiscovery runs a UDP :58866 broadcast discovery pass and persists matched IPs/versions.
func (f *LANFleet) SyncDiscovery(ctx context.Context, timeout time.Duration) ([]BroadcastDevice, int, error) {
	found, err := f.discover(ctx, timeout)
	if err != nil {
		return nil, 0, err
	}
	updated := 0
	if f.Store != nil {
		updated, err = f.Store.UpdateDiscovered(found)
		if err != nil {
			return found, updated, err
		}
	}
	f.mu.Lock()
	f.lastScanAt = time.Now()
	f.mu.Unlock()
	return found, updated, nil
}

func (f *LANFleet) ensureIPs(ctx context.Context) {
	if f.Store == nil {
		return
	}
	devs := f.Store.Devices()
	missingIP := false
	for _, d := range devs {
		if strings.TrimSpace(d.IP) == "" {
			missingIP = true
			break
		}
	}
	if !missingIP {
		return
	}
	f.mu.Lock()
	due := time.Since(f.lastScanAt) >= f.minScanGap
	f.mu.Unlock()
	if !due {
		return
	}
	_, _, _ = f.SyncDiscovery(ctx, 4*time.Second)
}

// Snapshot polls all configured cloud_devices.json vacuums concurrently over LAN TCP :58867
// and returns VacuumTelemetry entries compatible with homectl's REST API, Web UI, CLI, and MCP.
func (f *LANFleet) Snapshot(ctx context.Context) []VacuumTelemetry {
	if f.Store == nil {
		return []VacuumTelemetry{}
	}
	f.ensureIPs(ctx)
	devs := f.Store.Devices()
	if len(devs) == 0 {
		return []VacuumTelemetry{}
	}

	out := make([]VacuumTelemetry, len(devs))
	var wg sync.WaitGroup
	for i, d := range devs {
		wg.Add(1)
		go func(idx int, rec CloudDeviceRecord) {
			defer wg.Done()
			out[idx] = f.pollDevice(ctx, rec)
		}(i, d)
	}
	wg.Wait()
	return out
}

func (f *LANFleet) pollDevice(ctx context.Context, rec CloudDeviceRecord) VacuumTelemetry {
	t := VacuumTelemetry{
		DID:              rec.DUID,
		DUID:             rec.DUID,
		Name:             rec.Name,
		Model:            rec.Model,
		RemoteIP:         rec.IP,
		FirmwareVersion:  rec.FirmwareVersion,
		ProtocolVersion:  rec.LocalProtocol,
		HasPublicKey:     true,
		KeyRecoveryState: "lan_local_key",
	}
	if strings.TrimSpace(rec.IP) == "" {
		t.StateName = "awaiting_lan_discovery"
		return t
	}
	timeout := f.DialTimeout
	if timeout <= 0 {
		timeout = 4 * time.Second
	}
	rpcCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client := NewLocalTCPClient(rec.IP, rec.LocalKey, rec.LocalProtocol)
	if err := client.Connect(rpcCtx); err != nil {
		// Return last known metadata as offline rather than failing the fleet poll.
		t.Connected = false
		return t
	}
	defer client.Close()

	t.Connected = true
	t.ProtocolVersion = client.ProtocolVersion()
	t.LastSeen = time.Now().UTC().Format(time.RFC3339)

	res, err := client.SendRPC(rpcCtx, "get_status", nil)
	if err == nil && res != nil && len(res.Result) > 0 {
		applyStatusResult(&t, res.Result)
	}
	return t
}

func applyStatusResult(t *VacuumTelemetry, raw json.RawMessage) {
	// Roborock get_status returns either a single object or a 1-element array [{...}].
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		var arr []map[string]any
		if json.Unmarshal(raw, &arr) == nil && len(arr) > 0 {
			obj = arr[0]
		}
	}
	if obj == nil {
		return
	}
	if v, ok := anyToInt(obj["battery"]); ok {
		t.Battery = &v
	}
	if v, ok := anyToInt(obj["state"]); ok {
		t.State = &v
		t.StateName = VacuumStateName(v)
	}
	if v, ok := anyToInt(obj["error_code"]); ok {
		t.ErrorCode = &v
		t.ErrorName = VacuumErrorName(v)
	}
	if v, ok := anyToInt(obj["fan_power"]); ok {
		t.FanPower = &v
	}
	if v, ok := anyToInt(obj["water_box_mode"]); ok {
		t.WaterBoxMode = &v
	}
	if v, ok := anyToInt(obj["charge_status"]); ok {
		t.ChargeStatus = &v
	}
	if v, ok := anyToInt(obj["dry_status"]); ok {
		t.DryingStatus = &v
	}
}

func (f *LANFleet) resolveAndConnect(ctx context.Context, target string) (*CloudDeviceRecord, *LocalTCPClient, error) {
	if f.Store == nil {
		return nil, nil, errors.New("roborock: cloud device store is not initialized")
	}
	rec, ok := f.Store.Find(target)
	if !ok {
		return nil, nil, fmt.Errorf("roborock: unknown vacuum %q (run 'homectl roborock login' first)", target)
	}
	if strings.TrimSpace(rec.IP) == "" {
		_, _, _ = f.SyncDiscovery(ctx, 4*time.Second)
		rec, ok = f.Store.Find(target)
		if !ok || strings.TrimSpace(rec.IP) == "" {
			return nil, nil, fmt.Errorf("roborock: vacuum %q (%s) has no discovered LAN IP yet (ensure it is powered on and on the same subnet)", rec.Name, rec.DUID)
		}
	}
	timeout := f.DialTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client := NewLocalTCPClient(rec.IP, rec.LocalKey, rec.LocalProtocol)
	if err := client.Connect(dialCtx); err != nil {
		// Retry once after a fresh discovery pass in case DHCP changed the vacuum's IP.
		_, updated, _ := f.SyncDiscovery(ctx, 4*time.Second)
		if updated > 0 {
			if refreshed, ok2 := f.Store.Find(rec.DUID); ok2 && refreshed.IP != "" && refreshed.IP != rec.IP {
				rec = refreshed
				client = NewLocalTCPClient(rec.IP, rec.LocalKey, rec.LocalProtocol)
				dialCtx2, cancel2 := context.WithTimeout(ctx, timeout)
				defer cancel2()
				if err2 := client.Connect(dialCtx2); err2 == nil {
					return rec, client, nil
				}
			}
		}
		return nil, nil, fmt.Errorf("roborock: connect to %s (%s): %w", rec.Name, rec.IP, err)
	}
	return rec, client, nil
}

// Status queries live get_status and get_consumable directly over LAN TCP :58867.
func (f *LANFleet) Status(ctx context.Context, target string) (map[string]any, error) {
	rec, client, err := f.resolveAndConnect(ctx, target)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	statusRes, err := client.SendRPC(ctx, "get_status", nil)
	if err != nil {
		return nil, err
	}
	consRes, _ := client.SendRPC(ctx, "get_consumable", nil)
	out := map[string]any{
		"id":        rec.DUID,
		"name":      rec.Name,
		"model":     rec.Model,
		"ip":        rec.IP,
		"protocol":  client.ProtocolVersion(),
		"transport": "lan_tcp",
		"status":    statusRes.Result,
	}
	if consRes != nil && len(consRes.Result) > 0 {
		out["consumables"] = consRes.Result
	}
	return out, nil
}

// RoomEntry maps a vacuum segment ID to its human-readable cloud room name (if known).
type RoomEntry struct {
	SegmentID int    `json:"segment_id"`
	CloudID   string `json:"cloud_id,omitempty"`
	Name      string `json:"name,omitempty"`
}

// Rooms queries get_room_mapping directly over LAN TCP :58867 and enriches IDs with cloud room names.
func (f *LANFleet) Rooms(ctx context.Context, target string) ([]RoomEntry, json.RawMessage, error) {
	_, client, err := f.resolveAndConnect(ctx, target)
	if err != nil {
		return nil, nil, err
	}
	defer client.Close()

	res, err := client.SendRPC(ctx, "get_room_mapping", nil)
	if err != nil {
		return nil, nil, err
	}
	cloudRooms := f.Store.Rooms()
	var pairs [][]any
	_ = json.Unmarshal(res.Result, &pairs)
	entries := make([]RoomEntry, 0, len(pairs))
	for _, pair := range pairs {
		if len(pair) < 2 {
			continue
		}
		segID, ok := anyToInt(pair[0])
		if !ok {
			continue
		}
		cloudIDStr := stringFromAny(pair[1])
		entry := RoomEntry{SegmentID: segID, CloudID: cloudIDStr}
		if cid, ok := anyToInt(pair[1]); ok {
			entry.Name = cloudRooms[cid]
		}
		entries = append(entries, entry)
	}
	return entries, res.Result, nil
}

// ControlOptions configures optional parameters for LANFleet.ControlWithOptions.
type ControlOptions struct {
	Rooms     []int
	Repeat    int
	FanPower  int
	WaterMode int
}

// Control executes a vacuum action directly over LAN TCP :58867.
func (f *LANFleet) Control(ctx context.Context, target, action string, rooms []int, repeat int) (*RPCResponse, error) {
	return f.ControlWithOptions(ctx, target, action, ControlOptions{Rooms: rooms, Repeat: repeat})
}

// ControlWithOptions executes a vacuum action directly over LAN TCP :58867 with full options.
func (f *LANFleet) ControlWithOptions(ctx context.Context, target, action string, opts ControlOptions) (*RPCResponse, error) {
	_, client, err := f.resolveAndConnect(ctx, target)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	switch strings.ToLower(strings.TrimSpace(action)) {
	case "start", "clean":
		return client.SendRPC(ctx, "app_start", nil)
	case "pause":
		return client.SendRPC(ctx, "app_pause", nil)
	case "stop":
		return client.SendRPC(ctx, "app_stop", nil)
	case "dock", "home", "charge":
		return client.SendRPC(ctx, "app_charge", nil)
	case "find", "locate":
		return client.SendRPC(ctx, "find_me", nil)
	case "clean_rooms", "segment_clean":
		if len(opts.Rooms) == 0 {
			return nil, errors.New("roborock: clean_rooms requires at least one segment/room ID")
		}
		repeat := opts.Repeat
		if repeat <= 0 {
			repeat = 1
		}
		return client.SendRPC(ctx, "app_segment_clean", []any{map[string]any{
			"segments": opts.Rooms,
			"repeat":   repeat,
		}})
	case "fan_power":
		if opts.FanPower <= 0 {
			return nil, errors.New("roborock: fan_power requires a positive suction code (e.g. 101-105)")
		}
		return client.SendRPC(ctx, "set_custom_mode", []any{opts.FanPower})
	case "water_mode":
		if opts.WaterMode <= 0 {
			return nil, errors.New("roborock: water_mode requires a positive water mode code (e.g. 200-203)")
		}
		return client.SendRPC(ctx, "set_water_box_custom_mode", []any{opts.WaterMode})
	default:
		return nil, fmt.Errorf("roborock: unsupported action %q", action)
	}
}
