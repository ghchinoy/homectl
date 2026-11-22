package roborock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// RPCResponse wraps a decoded JSON-RPC reply from a vacuum (protocol 102).
type RPCResponse struct {
	ID     int64           `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  any             `json:"error,omitempty"`
}

// MQTTController sends active encrypted 1.0 RPC commands to vacuums via the embedded MQTT broker
// and correlates protocol-102 replies by JSON-RPC request ID.
type MQTTController struct {
	broker  *MQTTBroker
	creds   *CredentialStore
	nextID  atomic.Int64
	nextSeq atomic.Uint32

	mu      sync.Mutex
	waiters map[int64]chan map[string]any
}

// NewMQTTController attaches an active RPC controller to an embedded MQTTBroker.
func NewMQTTController(broker *MQTTBroker, creds *CredentialStore) *MQTTController {
	c := &MQTTController{
		broker:  broker,
		creds:   creds,
		waiters: make(map[int64]chan map[string]any),
	}
	c.nextID.Store(1000)
	c.nextSeq.Store(100000)
	if broker != nil {
		broker.SetRPCCallback(c.handleRPCResponse)
	}
	return c
}

func (c *MQTTController) handleRPCResponse(_ string, rpcID int64, resp map[string]any) {
	c.mu.Lock()
	ch, ok := c.waiters[rpcID]
	if ok {
		delete(c.waiters, rpcID)
	}
	c.mu.Unlock()
	if ok {
		select {
		case ch <- resp:
		default:
		}
	}
}

// SendRPC encodes and publishes a V1 JSON-RPC request (protocol 101) to the target vacuum
// (matched by DID, DUID, Name, or model suffix like "a144") and awaits its correlated reply.
func (c *MQTTController) SendRPC(ctx context.Context, target, method string, params any) (*RPCResponse, error) {
	if c.broker == nil || c.creds == nil {
		return nil, errors.New("roborock: controller not initialized")
	}
	dev, ok := c.creds.ResolveByNameOrID(target)
	if !ok {
		return nil, fmt.Errorf("roborock: unknown vacuum %q", target)
	}
	if dev.LocalKey == "" {
		return nil, fmt.Errorf("roborock: vacuum %q has no localKey yet (complete onboarding first)", target)
	}
	dt, ok := c.broker.ActiveDeviceTopic(dev.DID)
	if !ok {
		return nil, fmt.Errorf("roborock: vacuum %q (%s) has no active MQTT topic", dev.Name, dev.DID)
	}

	reqID := c.nextID.Add(1)
	if params == nil {
		params = []any{}
	}
	rpcObj := map[string]any{
		"id":     reqID,
		"method": method,
		"params": params,
	}
	rpcBytes, err := json.Marshal(rpcObj)
	if err != nil {
		return nil, err
	}
	nowTS := uint32(time.Now().Unix())
	envelopeBytes, err := json.Marshal(map[string]any{
		"t": nowTS,
		"dps": map[string]any{
			"101": string(rpcBytes),
		},
	})
	if err != nil {
		return nil, err
	}

	seq := c.nextSeq.Add(1)
	wire, err := EncodeMessage(Message{
		Version:   Version10,
		Seq:       seq,
		Random:    seq ^ 0x5A5A,
		Timestamp: nowTS,
		Protocol:  ProtoRPCRequest,
		Payload:   envelopeBytes,
	}, CodecOptions{
		LocalKey: dev.LocalKey,
		Prefixed: false,
	})
	if err != nil {
		return nil, err
	}

	ch := make(chan map[string]any, 1)
	c.mu.Lock()
	c.waiters[reqID] = ch
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.waiters, reqID)
		c.mu.Unlock()
	}()

	if err := c.broker.PublishInline(dt.topicOut(), wire, 0); err != nil {
		return nil, fmt.Errorf("roborock: publish rpc %s: %w", method, err)
	}

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("roborock: rpc %s timed out waiting for %s: %w", method, dev.Name, ctx.Err())
	case raw := <-ch:
		out := &RPCResponse{ID: reqID, Error: raw["error"]}
		if resVal, exists := raw["result"]; exists {
			b, _ := json.Marshal(resVal)
			out.Result = b
		}
		if out.Error != nil {
			return out, fmt.Errorf("roborock: rpc %s returned error: %v", method, out.Error)
		}
		return out, nil
	}
}

// High-level vacuum command helpers.
func (c *MQTTController) GetStatus(ctx context.Context, target string) (*RPCResponse, error) {
	return c.SendRPC(ctx, target, "get_status", []any{})
}

func (c *MQTTController) GetConsumables(ctx context.Context, target string) (*RPCResponse, error) {
	return c.SendRPC(ctx, target, "get_consumable", []any{})
}

func (c *MQTTController) GetRoomMapping(ctx context.Context, target string) (*RPCResponse, error) {
	return c.SendRPC(ctx, target, "get_room_mapping", []any{})
}

func (c *MQTTController) GetCleanSummary(ctx context.Context, target string) (*RPCResponse, error) {
	return c.SendRPC(ctx, target, "get_clean_summary", []any{})
}

func (c *MQTTController) StartClean(ctx context.Context, target string) (*RPCResponse, error) {
	return c.SendRPC(ctx, target, "app_start", []any{})
}

func (c *MQTTController) PauseClean(ctx context.Context, target string) (*RPCResponse, error) {
	return c.SendRPC(ctx, target, "app_pause", []any{})
}

func (c *MQTTController) StopClean(ctx context.Context, target string) (*RPCResponse, error) {
	return c.SendRPC(ctx, target, "app_stop", []any{})
}

func (c *MQTTController) ReturnToDock(ctx context.Context, target string) (*RPCResponse, error) {
	return c.SendRPC(ctx, target, "app_charge", []any{})
}

func (c *MQTTController) FindMe(ctx context.Context, target string) (*RPCResponse, error) {
	return c.SendRPC(ctx, target, "find_me", []any{})
}

func (c *MQTTController) CleanRooms(ctx context.Context, target string, segments []int, repeat int) (*RPCResponse, error) {
	if repeat <= 0 {
		repeat = 1
	}
	return c.SendRPC(ctx, target, "app_segment_clean", []any{
		map[string]any{
			"segments": segments,
			"repeat":   repeat,
		},
	})
}

func (c *MQTTController) SetFanPower(ctx context.Context, target string, fanPower int) (*RPCResponse, error) {
	return c.SendRPC(ctx, target, "set_custom_mode", []any{fanPower})
}

func (c *MQTTController) SetWaterMode(ctx context.Context, target string, waterMode int) (*RPCResponse, error) {
	return c.SendRPC(ctx, target, "set_water_box_custom_mode", []any{waterMode})
}

func (c *MQTTController) RequestMap(ctx context.Context, target string) (*RPCResponse, error) {
	return c.SendRPC(ctx, target, "get_map_v1", []any{})
}

// WebRTC Camera signaling RPC helpers (for Saros 10R a144 & Saros 20 a288).
func (c *MQTTController) GetCameraStatus(ctx context.Context, target string) (*RPCResponse, error) {
	return c.SendRPC(ctx, target, "get_camera_status", []any{})
}

func (c *MQTTController) SetCameraStatus(ctx context.Context, target string, status int) (*RPCResponse, error) {
	return c.SendRPC(ctx, target, "set_camera_status", []any{status})
}

func (c *MQTTController) StartCameraPreview(ctx context.Context, target string, params any) (*RPCResponse, error) {
	return c.SendRPC(ctx, target, "start_camera_preview", params)
}

func (c *MQTTController) GetDeviceSDP(ctx context.Context, target string, params any) (*RPCResponse, error) {
	return c.SendRPC(ctx, target, "get_device_sdp", params)
}
