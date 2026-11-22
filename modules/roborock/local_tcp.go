package roborock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const DefaultLocalTCPPort = 58867

// LocalTCPClient communicates directly with a Roborock vacuum over LAN TCP port 58867
// using either legacy 1.0 (AES-128-ECB) or modern L01 (AES-256-GCM with connect/ack nonces).
type LocalTCPClient struct {
	addr     string
	localKey string
	version  string

	mu           sync.Mutex
	conn         net.Conn
	connectNonce uint32
	ackNonce     *uint32
	seq          atomic.Uint32
	reqID        atomic.Int64
}

// NewLocalTCPClient constructs a direct LAN TCP client for host:port (defaults to :58867).
// If version is empty, Connect automatically negotiates between "1.0" and "L01".
func NewLocalTCPClient(host, localKey, version string) *LocalTCPClient {
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = fmt.Sprintf("%s:%d", host, DefaultLocalTCPPort)
	}
	c := &LocalTCPClient{
		addr:     host,
		localKey: localKey,
		version:  version,
	}
	c.seq.Store(1)
	c.reqID.Store(10000)
	return c
}

// ProtocolVersion returns the negotiated local protocol version ("1.0" or "L01").
func (c *LocalTCPClient) ProtocolVersion() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.version
}

// Connect dials the vacuum and negotiates HELLO_REQUEST(0) / HELLO_RESPONSE(1),
// trying the preferred version first (or 1.0 then L01 if unspecified).
func (c *LocalTCPClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	versions := []string{Version10, VersionL01}
	if c.version == VersionL01 {
		versions = []string{VersionL01, Version10}
	} else if c.version == Version10 {
		versions = []string{Version10, VersionL01}
	}

	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return fmt.Errorf("roborock: dial local tcp %s: %w", c.addr, err)
	}
	c.conn = conn
	c.connectNonce = uint32(10000 + (time.Now().UnixNano() % 22000))
	c.ackNonce = nil

	var lastErr error
	for _, ver := range versions {
		ack, helloErr := c.tryHelloLocked(ctx, conn, ver)
		if helloErr == nil {
			c.version = ver
			c.ackNonce = &ack
			return nil
		}
		lastErr = helloErr
	}
	_ = conn.Close()
	c.conn = nil
	return fmt.Errorf("roborock: local tcp hello negotiation failed on %s: %w", c.addr, lastErr)
}

func (c *LocalTCPClient) tryHelloLocked(ctx context.Context, conn net.Conn, version string) (uint32, error) {
	hello := Message{
		Version:   version,
		Seq:       c.seq.Add(1),
		Random:    c.connectNonce,
		Timestamp: uint32(time.Now().Unix()),
		Protocol:  ProtoHelloRequest,
	}
	wire, err := EncodeMessage(hello, CodecOptions{
		LocalKey:     c.localKey,
		Prefixed:     true,
		ConnectNonce: c.connectNonce,
	})
	if err != nil {
		return 0, err
	}

	helloDeadline := time.Now().Add(2500 * time.Millisecond)
	if dl, ok := ctx.Deadline(); ok && dl.Before(helloDeadline) {
		helloDeadline = dl
	}
	_ = conn.SetDeadline(helloDeadline)

	if _, err := conn.Write(wire); err != nil {
		return 0, fmt.Errorf("write %s hello: %w", version, err)
	}

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		return 0, fmt.Errorf("read %s hello response: %w", version, err)
	}
	msgs, _, err := DecodeMessages(buf[:n], CodecOptions{
		LocalKey:     c.localKey,
		Prefixed:     true,
		ConnectNonce: c.connectNonce,
	})
	if err != nil || len(msgs) == 0 {
		return 0, fmt.Errorf("decode %s hello response: %w", version, err)
	}
	for _, resp := range msgs {
		if resp.Protocol == ProtoHelloResponse {
			return resp.Random, nil
		}
	}
	return 0, fmt.Errorf("expected HELLO_RESPONSE(1) for %s, got %s", version, msgs[0].Protocol)
}

// SendRPC sends an encrypted GENERAL_REQUEST(4) JSON-RPC call over the established local TCP session.
// Per python-roborock V1Protocol, the inner DPS key is "101" (request) and replies arrive under "102"
// (with fallback checks for "4"/"5" for mock compatibility).
func (c *LocalTCPClient) SendRPC(ctx context.Context, method string, params any) (*RPCResponse, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil, errors.New("roborock: local tcp client not connected")
	}
	if params == nil {
		params = []any{}
	}
	id := c.reqID.Add(1)
	rpcBytes, err := json.Marshal(map[string]any{
		"id":     id,
		"method": method,
		"params": params,
	})
	if err != nil {
		return nil, err
	}
	nowTS := uint32(time.Now().Unix())
	envBytes, err := json.Marshal(map[string]any{
		"t":   nowTS,
		"dps": map[string]any{"101": string(rpcBytes)},
	})
	if err != nil {
		return nil, err
	}
	opts := CodecOptions{
		LocalKey:     c.localKey,
		Prefixed:     true,
		ConnectNonce: c.connectNonce,
		AckNonce:     c.ackNonce,
	}
	seq := c.seq.Add(1)
	wire, err := EncodeMessage(Message{
		Version:   c.version,
		Seq:       seq,
		Random:    seq ^ 0xA5A5,
		Timestamp: nowTS,
		Protocol:  ProtoGeneralRequest,
		Payload:   envBytes,
	}, opts)
	if err != nil {
		return nil, err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = c.conn.SetDeadline(deadline)
	} else {
		_ = c.conn.SetDeadline(time.Now().Add(5 * time.Second))
	}
	if _, err := c.conn.Write(wire); err != nil {
		return nil, err
	}

	var accum []byte
	buf := make([]byte, 8192)
	for {
		n, err := c.conn.Read(buf)
		if err != nil {
			return nil, err
		}
		accum = append(accum, buf[:n]...)
		msgs, rem, decErr := DecodeMessages(accum, opts)
		if decErr != nil {
			return nil, decErr
		}
		accum = rem
		for _, m := range msgs {
			if len(m.Payload) == 0 {
				continue
			}
			var obj map[string]any
			if json.Unmarshal(m.Payload, &obj) != nil {
				continue
			}
			dps, _ := obj["dps"].(map[string]any)
			for _, k := range []string{"102", "4", "5"} {
				if rpc, ok := extractRPCFromDPS(dps, k); ok {
					if gotID, ok := anyToInt(rpc["id"]); ok && int64(gotID) == id {
						res := &RPCResponse{ID: id, Error: rpc["error"]}
						if v, exists := rpc["result"]; exists {
							b, _ := json.Marshal(v)
							res.Result = b
						}
						return res, nil
					}
				}
			}
		}
	}
}

// Close closes the underlying TCP socket.
func (c *LocalTCPClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	return err
}
