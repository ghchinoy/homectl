package roborock

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// VacuumTelemetry holds the latest observed state and instrumentation metrics for one vacuum.
type VacuumTelemetry struct {
	DID               string `json:"did"`
	DUID              string `json:"duid"`
	Name              string `json:"name"`
	Model             string `json:"model"`
	Connected         bool   `json:"connected"`
	RemoteIP          string `json:"remote_ip,omitempty"`
	FirmwareVersion   string `json:"firmware_version,omitempty"`
	ProtocolVersion   string `json:"protocol_version,omitempty"`
	Battery           *int   `json:"battery,omitempty"`
	State             *int   `json:"state,omitempty"`
	StateName         string `json:"state_name,omitempty"`
	ErrorCode         *int   `json:"error_code,omitempty"`
	ErrorName         string `json:"error_name,omitempty"`
	FanPower          *int   `json:"fan_power,omitempty"`
	WaterBoxMode      *int   `json:"water_box_mode,omitempty"`
	ChargeStatus      *int   `json:"charge_status,omitempty"`
	DryingStatus      *int   `json:"drying_status,omitempty"`
	MainBrushWorkTime *int   `json:"main_brush_work_time,omitempty"`
	SideBrushWorkTime *int   `json:"side_brush_work_time,omitempty"`
	FilterWorkTime    *int   `json:"filter_work_time,omitempty"`
	HasPublicKey      bool   `json:"has_public_key"`
	KeyRecoveryState  string `json:"key_recovery_state,omitempty"`
	QuerySamples      int    `json:"query_samples"`
	HeaderSamples     int    `json:"header_samples"`
	LastRPCMethod     string `json:"last_rpc_method,omitempty"`
	LastSeen          string `json:"last_seen,omitempty"`
	LastMapAt         string `json:"last_map_at,omitempty"`
	LastMapBytes      int    `json:"last_map_bytes,omitempty"`
	MQTTMessageCount  int64  `json:"mqtt_message_count"`
}

// TelemetryEvent represents a single real-time instrumentation event emitted by the local stack.
type TelemetryEvent struct {
	Timestamp string         `json:"timestamp"`
	Kind      string         `json:"kind"` // mqtt_dps | mqtt_rpc_req | mqtt_rpc_resp | mqtt_ota | mqtt_map | connect | disconnect | http | key_recovery
	DID       string         `json:"did,omitempty"`
	DUID      string         `json:"duid,omitempty"`
	Topic     string         `json:"topic,omitempty"`
	Protocol  string         `json:"protocol,omitempty"`
	Summary   string         `json:"summary,omitempty"`
	Data      map[string]any `json:"data,omitempty"`
}

// VacuumStateName maps Roborock V1 state integer codes to human-readable descriptions.
func VacuumStateName(code int) string {
	switch code {
	case 1:
		return "starting"
	case 2:
		return "charger_disconnected"
	case 3:
		return "idle"
	case 4:
		return "remote_control"
	case 5:
		return "cleaning"
	case 6:
		return "returning_home"
	case 7:
		return "manual_mode"
	case 8:
		return "charging"
	case 9:
		return "charging_problem"
	case 10:
		return "paused"
	case 11:
		return "spot_cleaning"
	case 12:
		return "error"
	case 13:
		return "shutting_down"
	case 14:
		return "updating"
	case 15:
		return "docking"
	case 16:
		return "going_to_target"
	case 17:
		return "zoned_cleaning"
	case 18:
		return "segment_cleaning"
	case 22:
		return "emptying_bin"
	case 23:
		return "washing_mop"
	case 26:
		return "going_to_wash_mop"
	case 28:
		return "in_call"
	case 29:
		return "mapping"
	case 100:
		return "charging_complete"
	default:
		return fmt.Sprintf("state_%d", code)
	}
}

// VacuumErrorName maps Roborock V1 error codes to human-readable descriptions.
func VacuumErrorName(code int) string {
	switch code {
	case 0:
		return "none"
	case 1:
		return "lidar_blocked"
	case 2:
		return "bumper_stuck"
	case 3:
		return "wheels_suspended"
	case 4:
		return "cliff_sensor_error"
	case 5:
		return "main_brush_jammed"
	case 6:
		return "side_brush_jammed"
	case 7:
		return "wheels_jammed"
	case 8:
		return "robot_trapped"
	case 9:
		return "no_dustbin"
	case 12:
		return "low_battery"
	case 13:
		return "charging_error"
	case 14:
		return "battery_error"
	case 15:
		return "wall_sensor_dirty"
	case 16:
		return "robot_tilted"
	case 17:
		return "side_brush_error"
	case 18:
		return "fan_error"
	case 21:
		return "vertical_bumper_pressed"
	case 22:
		return "dock_locator_error"
	case 23:
		return "return_to_dock_fail"
	case 24:
		return "nogo_zone_detected"
	case 27:
		return "vibra_rise_jammed"
	case 28:
		return "robot_on_carpet"
	case 29:
		return "filter_blocked"
	case 30:
		return "invisible_wall_detected"
	case 34:
		return "clean_water_tank_empty"
	case 35:
		return "dirty_water_tank_full"
	case 38:
		return "water_filter_blocked"
	case 39:
		return "clean_tank_missing"
	default:
		return fmt.Sprintf("error_%d", code)
	}
}

// TelemetryHub aggregates live per-vacuum telemetry, map frames, recent event history, and pub/sub streams.
type TelemetryHub struct {
	mu           sync.RWMutex
	vacuums      map[string]*VacuumTelemetry // keyed by DID (or DUID if DID empty)
	maps         map[string][]byte           // latest raw map payload by DID
	mapParts     map[string][]byte           // in-progress protocol-300 map buffer by DID
	events       []TelemetryEvent
	maxEvents    int
	subs         map[int]chan TelemetryEvent
	nextSubID    int
	tracePath    string
	traceEnabled bool
}

// NewTelemetryHub constructs a TelemetryHub with a bounded ring buffer and optional JSONL trace path.
func NewTelemetryHub(maxEvents int, tracePath string) *TelemetryHub {
	if maxEvents <= 0 {
		maxEvents = 1000
	}
	return &TelemetryHub{
		vacuums:      make(map[string]*VacuumTelemetry),
		maps:         make(map[string][]byte),
		mapParts:     make(map[string][]byte),
		events:       make([]TelemetryEvent, 0, maxEvents),
		maxEvents:    maxEvents,
		subs:         make(map[int]chan TelemetryEvent),
		tracePath:    strings.TrimSpace(tracePath),
		traceEnabled: strings.TrimSpace(tracePath) != "",
	}
}

// Subscribe registers a live TelemetryEvent listener. Call the returned cancel func when done.
func (h *TelemetryHub) Subscribe() (<-chan TelemetryEvent, func()) {
	ch := make(chan TelemetryEvent, 64)
	h.mu.Lock()
	id := h.nextSubID
	h.nextSubID++
	h.subs[id] = ch
	h.mu.Unlock()

	cancel := func() {
		h.mu.Lock()
		if existing, ok := h.subs[id]; ok {
			delete(h.subs, id)
			close(existing)
		}
		h.mu.Unlock()
	}
	return ch, cancel
}

func (h *TelemetryHub) ensureLocked(did string) *VacuumTelemetry {
	did = strings.TrimSpace(did)
	v, ok := h.vacuums[did]
	if !ok {
		v = &VacuumTelemetry{DID: did, DUID: did}
		h.vacuums[did] = v
	}
	return v
}

// Emit appends an event to history, fans out to subscribers (non-blocking), and optionally appends to JSONL trace.
func (h *TelemetryHub) Emit(ev TelemetryEvent) {
	if ev.Timestamp == "" {
		ev.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	h.mu.Lock()
	if len(h.events) >= h.maxEvents {
		copy(h.events, h.events[1:])
		h.events[len(h.events)-1] = ev
	} else {
		h.events = append(h.events, ev)
	}
	for _, ch := range h.subs {
		select {
		case ch <- ev:
		default:
			// Drop for slow consumers rather than stalling MQTT/HTTP handlers.
		}
	}
	tracePath := h.tracePath
	traceEnabled := h.traceEnabled
	h.mu.Unlock()

	if traceEnabled && tracePath != "" {
		if raw, err := json.Marshal(ev); err == nil {
			_ = os.MkdirAll(filepath.Dir(tracePath), 0700)
			if f, err := os.OpenFile(tracePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600); err == nil {
				_, _ = f.Write(append(raw, '\n'))
				_ = f.Close()
			}
		}
	}
}

// RecordConnection updates live MQTT connection state for a device.
func (h *TelemetryHub) RecordConnection(did, duid, name, model, remoteIP string, connected bool) {
	if did == "" {
		did = duid
	}
	if did == "" {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	h.mu.Lock()
	v := h.ensureLocked(did)
	if duid != "" {
		v.DUID = duid
	}
	if name != "" {
		v.Name = name
	}
	if model != "" {
		v.Model = model
	}
	if remoteIP != "" {
		v.RemoteIP = remoteIP
	}
	v.Connected = connected
	v.LastSeen = now
	h.mu.Unlock()

	kind := "disconnect"
	summary := "MQTT disconnected"
	if connected {
		kind = "connect"
		summary = "MQTT connected"
	}
	h.Emit(TelemetryEvent{
		Kind:    kind,
		DID:     did,
		DUID:    duid,
		Summary: summary,
		Data:    map[string]any{"remote_ip": remoteIP, "connected": connected},
	})
}

// RecordDecodedMessage ingests a decoded Roborock MQTT/TCP frame and updates live device telemetry.
func (h *TelemetryHub) RecordDecodedMessage(did, duid, topic string, msg Message) {
	if did == "" {
		did = duid
	}
	now := time.Now().UTC().Format(time.RFC3339)

	h.mu.Lock()
	v := h.ensureLocked(did)
	if duid != "" {
		v.DUID = duid
	}
	v.ProtocolVersion = msg.Version
	v.LastSeen = now
	v.MQTTMessageCount++

	switch msg.Protocol {
	case ProtoMapResponsePart:
		h.mapParts[did] = append(h.mapParts[did], msg.Payload...)
		h.mu.Unlock()
		h.Emit(TelemetryEvent{
			Kind:     "mqtt_map",
			DID:      did,
			DUID:     duid,
			Topic:    topic,
			Protocol: msg.Protocol.String(),
			Summary:  fmt.Sprintf("map chunk (%d bytes)", len(msg.Payload)),
		})
		return
	case ProtoMapResponse:
		combined := append(h.mapParts[did], msg.Payload...)
		delete(h.mapParts, did)
		h.maps[did] = combined
		v.LastMapAt = now
		v.LastMapBytes = len(combined)
		h.mu.Unlock()
		h.Emit(TelemetryEvent{
			Kind:     "mqtt_map",
			DID:      did,
			DUID:     duid,
			Topic:    topic,
			Protocol: msg.Protocol.String(),
			Summary:  fmt.Sprintf("map complete (%d bytes)", len(combined)),
		})
		return
	}

	var payloadObj map[string]any
	if len(msg.Payload) > 0 {
		_ = json.Unmarshal(msg.Payload, &payloadObj)
	}

	// Protocol 203: firmware version report {"mqttOtaData":{"mqttOtaFirmwareVersion":{"version":"02.52.86"}}}
	if msg.Protocol == ProtoOTAFirmwareReport {
		if fv := extractFirmwareVersion(payloadObj); fv != "" {
			v.FirmwareVersion = fv
			h.mu.Unlock()
			h.Emit(TelemetryEvent{
				Kind:     "mqtt_ota",
				DID:      did,
				DUID:     duid,
				Topic:    topic,
				Protocol: msg.Protocol.String(),
				Summary:  "firmware " + fv,
				Data:     map[string]any{"firmware_version": fv},
			})
			return
		}
	}

	// Parse DPS map when present (standard V1 telemetry and RPC envelopes).
	if dps, ok := payloadObj["dps"].(map[string]any); ok {
		h.applyDPSLocked(v, dps)
		if rpcReq, ok := extractRPCFromDPS(dps, "101"); ok {
			if m, _ := rpcReq["method"].(string); m != "" {
				v.LastRPCMethod = m
			}
			h.mu.Unlock()
			h.Emit(TelemetryEvent{
				Kind:     "mqtt_rpc_req",
				DID:      did,
				DUID:     duid,
				Topic:    topic,
				Protocol: msg.Protocol.String(),
				Summary:  fmt.Sprintf("RPC req %v", rpcReq["method"]),
				Data:     rpcReq,
			})
			return
		}
		if rpcResp, ok := extractRPCFromDPS(dps, "102"); ok {
			h.applyRPCStatusResultLocked(v, rpcResp)
			h.mu.Unlock()
			h.Emit(TelemetryEvent{
				Kind:     "mqtt_rpc_resp",
				DID:      did,
				DUID:     duid,
				Topic:    topic,
				Protocol: msg.Protocol.String(),
				Summary:  fmt.Sprintf("RPC resp id=%v", rpcResp["id"]),
				Data:     rpcResp,
			})
			return
		}
		h.mu.Unlock()
		h.Emit(TelemetryEvent{
			Kind:     "mqtt_dps",
			DID:      did,
			DUID:     duid,
			Topic:    topic,
			Protocol: msg.Protocol.String(),
			Summary:  formatDPSSummary(dps),
			Data:     dps,
		})
		return
	}

	h.mu.Unlock()
	h.Emit(TelemetryEvent{
		Kind:     "mqtt_dps",
		DID:      did,
		DUID:     duid,
		Topic:    topic,
		Protocol: msg.Protocol.String(),
		Summary:  fmt.Sprintf("%s (%d bytes)", msg.Protocol.String(), len(msg.Payload)),
	})
}

func (h *TelemetryHub) applyDPSLocked(v *VacuumTelemetry, dps map[string]any) {
	for k, raw := range dps {
		dpID, err := strconv.Atoi(k)
		if err != nil {
			continue
		}
		val, ok := anyToInt(raw)
		if !ok {
			continue
		}
		switch DataProtocol(dpID) {
		case DPErrorCode:
			v.ErrorCode = &val
			v.ErrorName = VacuumErrorName(val)
		case DPState:
			v.State = &val
			v.StateName = VacuumStateName(val)
		case DPBattery:
			v.Battery = &val
		case DPFanPower:
			v.FanPower = &val
		case DPWaterBoxMode:
			v.WaterBoxMode = &val
		case DPMainBrushWorkTime:
			v.MainBrushWorkTime = &val
		case DPSideBrushWorkTime:
			v.SideBrushWorkTime = &val
		case DPFilterWorkTime:
			v.FilterWorkTime = &val
		case DPChargeStatus:
			v.ChargeStatus = &val
		case DPDryingStatus:
			v.DryingStatus = &val
		}
	}
}

func (h *TelemetryHub) applyRPCStatusResultLocked(v *VacuumTelemetry, rpcResp map[string]any) {
	arr, ok := rpcResp["result"].([]any)
	if !ok || len(arr) == 0 {
		return
	}
	obj, ok := arr[0].(map[string]any)
	if !ok {
		return
	}
	if val, ok := anyToInt(obj["battery"]); ok {
		v.Battery = &val
	}
	if val, ok := anyToInt(obj["state"]); ok {
		v.State = &val
		v.StateName = VacuumStateName(val)
	}
	if val, ok := anyToInt(obj["error_code"]); ok {
		v.ErrorCode = &val
		v.ErrorName = VacuumErrorName(val)
	}
	if val, ok := anyToInt(obj["fan_power"]); ok {
		v.FanPower = &val
	}
	if val, ok := anyToInt(obj["water_box_mode"]); ok {
		v.WaterBoxMode = &val
	}
	if val, ok := anyToInt(obj["main_brush_work_time"]); ok {
		v.MainBrushWorkTime = &val
	}
	if val, ok := anyToInt(obj["side_brush_work_time"]); ok {
		v.SideBrushWorkTime = &val
	}
	if val, ok := anyToInt(obj["filter_work_time"]); ok {
		v.FilterWorkTime = &val
	}
}

// Snapshot merges CredentialStore and KeyStore metadata with live TelemetryHub observations.
func (h *TelemetryHub) Snapshot(cs *CredentialStore, ks *KeyStore) []VacuumTelemetry {
	h.mu.RLock()
	merged := make(map[string]VacuumTelemetry, len(h.vacuums))
	for did, v := range h.vacuums {
		merged[did] = *v
	}
	h.mu.RUnlock()

	if cs != nil {
		for _, dev := range cs.ListDevices() {
			key := dev.DID
			if key == "" {
				key = dev.DUID
			}
			if key == "" {
				continue
			}
			cur := merged[key]
			cur.DID = dev.DID
			if cur.DID == "" {
				cur.DID = key
			}
			cur.DUID = dev.DUID
			if cur.DUID == "" {
				cur.DUID = key
			}
			if dev.Name != "" {
				cur.Name = dev.Name
			}
			if dev.Model != "" {
				cur.Model = dev.Model
			}
			if cur.Name == "" && cur.Model != "" {
				cur.Name = DefaultProductName(cur.Model)
			}
			if dev.FirmwareVersion != "" && cur.FirmwareVersion == "" {
				cur.FirmwareVersion = dev.FirmwareVersion
			}
			if dev.LastIP != "" && cur.RemoteIP == "" {
				cur.RemoteIP = dev.LastIP
			}
			merged[key] = cur
		}
	}

	if ks != nil {
		for did, rec := range ks.RecordSnapshot() {
			cur := merged[did]
			if cur.DID == "" {
				cur.DID = did
			}
			if cur.DUID == "" {
				cur.DUID = did
			}
			if cur.Model == "" && rec.Model != "" {
				cur.Model = rec.Model
			}
			if cur.Name == "" && cur.Model != "" {
				cur.Name = DefaultProductName(cur.Model)
			}
			cur.HasPublicKey = rec.ModulusHex != ""
			cur.QuerySamples = len(rec.Samples)
			cur.HeaderSamples = len(rec.HeaderSamples)
			if rec.Recovery != nil {
				cur.KeyRecoveryState = rec.Recovery.State
			} else if cur.HasPublicKey {
				cur.KeyRecoveryState = "recovered"
			} else {
				cur.KeyRecoveryState = "missing"
			}
			merged[did] = cur
		}
	}

	out := make([]VacuumTelemetry, 0, len(merged))
	for _, v := range merged {
		if v.Name == "" {
			v.Name = DefaultProductName(v.Model)
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].DID < out[j].DID
	})
	return out
}

// RecentEvents returns up to limit most recent telemetry events (optionally filtered by DID/DUID).
func (h *TelemetryHub) RecentEvents(did string, limit int) []TelemetryEvent {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if limit <= 0 || limit > len(h.events) {
		limit = len(h.events)
	}
	var out []TelemetryEvent
	for i := len(h.events) - 1; i >= 0 && len(out) < limit; i-- {
		ev := h.events[i]
		if did != "" && ev.DID != did && ev.DUID != did {
			continue
		}
		out = append(out, ev)
	}
	// Reverse to chronological order.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// LatestMap returns the most recent complete map payload captured for did.
func (h *TelemetryHub) LatestMap(did string) ([]byte, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	b, ok := h.maps[did]
	if !ok {
		return nil, false
	}
	cp := make([]byte, len(b))
	copy(cp, b)
	return cp, true
}

func extractFirmwareVersion(obj map[string]any) string {
	ota, ok := obj["mqttOtaData"].(map[string]any)
	if !ok {
		return ""
	}
	fw, ok := ota["mqttOtaFirmwareVersion"].(map[string]any)
	if !ok {
		return ""
	}
	ver, _ := fw["version"].(string)
	return strings.TrimSpace(ver)
}

func extractRPCFromDPS(dps map[string]any, key string) (map[string]any, bool) {
	raw, ok := dps[key]
	if !ok {
		return nil, false
	}
	switch v := raw.(type) {
	case map[string]any:
		return v, true
	case string:
		var parsed map[string]any
		if err := json.Unmarshal([]byte(v), &parsed); err == nil && parsed != nil {
			return parsed, true
		}
	}
	return nil, false
}

func anyToInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		return int(i), err == nil
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(n))
		return i, err == nil
	default:
		return 0, false
	}
}

func formatDPSSummary(dps map[string]any) string {
	keys := make([]string, 0, len(dps))
	for k := range dps {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%v", k, dps[k]))
	}
	return strings.Join(parts, " ")
}
