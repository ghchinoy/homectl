
package leap

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// Client represents a LEAP client connection to a Lutron Bridge
type Client struct {
	addr       string
	lsConfig  *tls.Config
	conn       *tls.Conn
	reader     *bufio.Reader
	mu         sync.Mutex
	tagCounter int
}

// Message represents a basic LEAP message structure
type Message struct {
	CommuniqueType string          `json:"CommuniqueType"`
	Header         Header          `json:"Header"`
	Body           json.RawMessage `json:"Body,omitempty"`
}

type Header struct {
	Url        string `json:"Url"`
	StatusCode string `json:"StatusCode,omitempty"`
	ClientTag  string `json:"ClientTag,omitempty"`
}

// NewClient creates a new LEAP client with the provided certificates
func NewClient(addr, certFile, keyFile, caFile string) (*Client, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load client cert: %w", err)
	}

	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA cert: %w", err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	lsConfig := &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            caCertPool,
		InsecureSkipVerify: true,
	}

	return &Client{
		addr:      addr,
		lsConfig: tlsConfig,
	},
	nil
}

// Connect opens the TLS connection to the bridge
func (c *Client) Connect() error {
	conn, err := tls.Dial("tcp", c.addr, c.tlsConfig)
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}
	c.conn = conn
	c.reader = bufio.NewReader(conn)
	return nil
}

// Close closes the connection
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// RawRequest sends a JSON request and waits for a single line response
func (c *Client) RawRequest(req Message) (Message, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := json.Marshal(req)
	if err != nil {
		return Message{}, err
	}

	_, err = c.conn.Write(append(data, '\n'))
	if err != nil {
		return Message{}, err
	}

	line, err := c.reader.ReadBytes('\n')
	if err != nil {
		return Message{}, err
	}

	var resp Message
	err = json.Unmarshal(line, &resp)
	return resp, err
}

// Read sends a ReadRequest to the specified URL
func (c *Client) Read(url string) (Message, error) {
	req := Message{
		CommuniqueType: "ReadRequest",
		Header: Header{
			Url: url,
		},
	}
	return c.RawRequest(req)
}
