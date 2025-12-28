
package leap

import (
	"bufio"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// Client represents a LEAP client connection to a Lutron Bridge
type Client struct {
	addr      string
	tlsConfig *tls.Config
	conn      *tls.Conn
	reader    *bufio.Reader
	mu        sync.Mutex
}

// Message represents a basic LEAP message structure
type Message struct {
	CommuniqueType string          `json:"CommuniqueType"`
	Header         Header          `json:"Header"`
	Body           json.RawMessage `json:"Body,omitempty"`
}

type Header struct {
	Url             string `json:"Url"`
	StatusCode      string `json:"StatusCode,omitempty"`
	ClientTag       string `json:"ClientTag,omitempty"`
	MessageBodyType string `json:"MessageBodyType,omitempty"`
}

// Area represents a Lutron Area
type Area struct {
	Href string `json:"href"`
	Name string `json:"Name"`
}

type AreaResponse struct {
	Areas []Area `json:"Areas"`
}

// Device represents a Lutron Device
type Device struct {
	Href           string   `json:"href"`
	Name           string   `json:"Name"`
	DeviceType     string   `json:"DeviceType"`
	SerialNumber   int      `json:"SerialNumber,omitempty"`
	ModelNumber    string   `json:"ModelNumber,omitempty"`
	LocalZones     []Zone   `json:"LocalZones,omitempty"`
	AssociatedArea struct {
		Href string `json:"href"`
	} `json:"AssociatedArea,omitempty"`
}

type Zone struct {
	Href string `json:"href"`
}

type DeviceResponse struct {
	Devices []Device `json:"Devices"`
}

type Command struct {
	CommandType string `json:"CommandType"`
	Parameter   []Parameter `json:"Parameter"`
}

type Parameter struct {
	Type  string  `json:"Type"`
	Value float64 `json:"Value"`
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

		tlsConfig := &tls.Config{

			Certificates:       []tls.Certificate{cert},

			RootCAs:            caCertPool,

			InsecureSkipVerify: true,

		}

	

		return &Client{

			addr:      addr,

			tlsConfig: tlsConfig,

		}, nil
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

// Request sends a JSON request and waits for the specific ReadResponse or CreateResponse
func (c *Client) Request(req Message) (Message, error) {
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

	for {
		c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		line, err := c.reader.ReadBytes('\n')
		if err != nil {
			return Message{}, err
		}

		var resp Message
		if err := json.Unmarshal(line, &resp); err != nil {
			continue
		}

		// Handle responses for the requested URL
		if resp.Header.Url == req.Header.Url {
			return resp, nil
		}
	}
}

// GetAreas retrieves all areas
func (c *Client) GetAreas() ([]Area, error) {
	req := Message{
		CommuniqueType: "ReadRequest",
		Header: Header{
			Url: "/area",
		},
	}
	resp, err := c.Request(req)
	if err != nil {
		return nil, err
	}
	var body AreaResponse
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		return nil, err
	}
	return body.Areas, nil
}

// GetDevices retrieves all devices
func (c *Client) GetDevices() ([]Device, error) {
	req := Message{
		CommuniqueType: "ReadRequest",
		Header: Header{
			Url: "/device",
		},
	}
	resp, err := c.Request(req)
	if err != nil {
		return nil, err
	}
	var body DeviceResponse
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		return nil, err
	}
	return body.Devices, nil
}

// SetLevel sets the dimming level for a zone (0-100)
func (c *Client) SetLevel(zoneHref string, level float64) error {
	url := fmt.Sprintf("%s/commandprocessor", zoneHref)
	cmd := Command{
		CommandType: "GoToLevel",
		Parameter: []Parameter{
			{Type: "Level", Value: level},
		},
	}
	
	body, _ := json.Marshal(cmd)
	req := Message{
		CommuniqueType: "CreateRequest",
		Header: Header{
			Url: url,
		},
		Body: body,
	}
	
	_, err := c.Request(req)
	return err
}
