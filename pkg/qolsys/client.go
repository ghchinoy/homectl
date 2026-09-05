// Package qolsys provides secure WebSocket client communication with Qolsys IQ Panel alarm systems.
package qolsys

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"sync"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// Client represents a connection to a Qolsys IQ Panel
type Client struct {
	addr  string
	token string
	conn  *websocket.Conn
	mu    sync.Mutex

	// OnEvent is called when a message is received from the panel
	OnEvent func(msg map[string]interface{})
}

// NewClient creates a new Qolsys client
func NewClient(addr, token string) *Client {
	return &Client{
		addr:  addr,
		token: token,
	}
}

// Connect opens a secure websocket connection to the panel
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Panels use self-signed certs
	hc := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	opts := &websocket.DialOptions{
		HTTPClient: hc,
	}

	url := fmt.Sprintf("wss://%s:12345", c.addr)
	conn, _, err := websocket.Dial(ctx, url, opts)
	if err != nil {
		return fmt.Errorf("failed to dial Qolsys panel: %w", err)
	}

	c.conn = conn
	return nil
}

// ReadLoop starts a loop to read messages from the panel
func (c *Client) ReadLoop(ctx context.Context) error {
	for {
		var msg map[string]interface{}
		err := wsjson.Read(ctx, c.conn, &msg)
		if err != nil {
			return err
		}

		if c.OnEvent != nil {
			c.OnEvent(msg)
		}
	}
}

// Send sends a command to the panel
func (c *Client) Send(ctx context.Context, action string, params map[string]interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	payload := map[string]interface{}{
		"action":   action,
		"user_pin": c.token, // Some versions use token as pin
		"version":  1,
		"nonce":    fmt.Sprintf("%d", time.Now().Unix()),
		"source":   "homectl",
	}
	for k, v := range params {
		payload[k] = v
	}

	return wsjson.Write(ctx, c.conn, payload)
}

// Close closes the connection
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return c.conn.Close(websocket.StatusNormalClosure, "closing")
	}
	return nil
}
