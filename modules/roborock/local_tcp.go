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
func NewLocalTCPClient(host, localKey, version string) *LocalTCPClient {
	if _, _, err := net.SplitHostPort(host); err != nil {
		host = fmt.Sprintf("%s:%d", host, DefaultLocalTCPPort)
	}
	if version == "" {
		version = VersionL01
	}
	c := &LocalTCPClient{
		addr:     host,
		localKey: localKey,
		version:  version,
	}
	c.seq.Store(1)
	c.reqID.Store(100)
	return c
}

// Connect dials the vacuum and performs the HELLO_REQUEST(0) / HELLO_RESPONSE(1) handshake.
func (c *LocalTCPClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return fmt.Errorf("roborock: dial local tcp %s: %w", c.addr, err)
	}
	c.conn = conn
	c.connectNonce = uint32(time.Now().UnixNano() & 0x7FFFFFFF)
	c.ackNonce = nil

	hello := Message{
		Version:   c.version,
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
		_ = conn.Close()
		c.conn = nil
		return err
	}
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	}
	if _, err := conn.Write(wire); err != nil {
		_ = conn.Close()
		c.conn = nil
		return fmt.Errorf("roborock: write local tcp hello: %w", err)
	}

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		_ = conn.Close()
		c.conn = nil
		return fmt.Errorf("roborock: read local tcp hello response: %w", err)
	}
	msgs, _, err := DecodeMessages(buf[:n], CodecOptions{
		LocalKey:     c.localKey,
		Prefixed:     true,
		ConnectNonce: c.connectNonce,
	})
	if err != nil || len(msgs) == 0 {
		_ = conn.Close()
		c.conn = nil
		return fmt.Errorf("roborock: decode local tcp hello response: %w", err)
	}
	resp := msgs[0]
	if resp.Protocol != ProtoHelloResponse {
		_ = conn.Close()
		c.conn = nil
		return fmt.Errorf("roborock: expected HELLO_RESPONSE(1), got %s", resp.Protocol)
	}
	ack := resp.Random
	c.ackNonce = &ack
	return nil
}

// SendRPC sends an encrypted GENERAL_REQUEST(4) JSON-RPC call over the established local TCP session.
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
		"dps": map[string]any{"4": string(rpcBytes)},
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
			for _, k := range []string{"4", "5", "102"} {
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
