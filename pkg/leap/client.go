package leap

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ghchinoy/homectl/pkg/config"
	"github.com/ghchinoy/homectl/pkg/discovery"
	"github.com/grandcat/zeroconf"
)

// DiscoveryProvider implements discovery.Provider for Lutron
type DiscoveryProvider struct{}

func (p *DiscoveryProvider) Name() string { return "lutron" }

func (p *DiscoveryProvider) Discover(ctx context.Context) ([]discovery.Device, error) {
	deadline, ok := ctx.Deadline()
	timeout := 5 * time.Second
	if ok {
		timeout = time.Until(deadline)
	}

	bridges, err := Discover(timeout)
	if err != nil {
		return nil, err
	}

	var devices []discovery.Device
	for _, b := range bridges {
		devices = append(devices, discovery.Device{
			ID:       b.Name, // Using name as ID for Lutron bridge discovery for now
			Name:     b.Name,
			IP:       b.IP,
			Provider: "lutron",
			Type:     "Bridge",
		})
	}
	return devices, nil
}


var (
	tagCounter uint64
	leapLogger *log.Logger
)

func nextTag() string {
	return fmt.Sprintf("%d", atomic.AddUint64(&tagCounter, 1))
}

func init() {
	config.EnsureDir()
	f, _ := os.OpenFile(config.GetPath("leap.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if f != nil {
		leapLogger = log.New(f, "LEAP: ", log.LstdFlags)
	}
}

// Bridge represents a discovered Lutron Bridge
type Bridge struct {
	Name string `json:"Name"`
	IP   string `json:"IP"`
}

// Discover performs mDNS discovery to find Lutron bridges
func Discover(timeout time.Duration) ([]Bridge, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, err
	}

	entries := make(chan *zeroconf.ServiceEntry)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err = resolver.Browse(ctx, "_leap._tcp", "local.", entries)
	if err != nil {
		return nil, err
	}

	var bridges []Bridge
	foundIPs := make(map[string]bool)

	for entry := range entries {
		var ip string
		if len(entry.AddrIPv4) > 0 {
			ip = entry.AddrIPv4[0].String()
		} else if len(entry.AddrIPv6) > 0 {
			ip = entry.AddrIPv6[0].String()
		}

		if ip == "" || foundIPs[ip] {
			continue
		}

		bridges = append(bridges, Bridge{
			Name: entry.Instance,
			IP:   ip,
		})
		foundIPs[ip] = true
	}

	return bridges, nil
}

func cacheFile() string {
	return config.GetPath("lutron_cache.json")
}

// SaveCache persists discovered bridges to a local file
func SaveCache(bridges []Bridge) error {
	data, err := json.MarshalIndent(bridges, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cacheFile(), data, 0644)
}

// LoadCache retrieves previously discovered bridges from the local file
func LoadCache() ([]Bridge, error) {
	data, err := os.ReadFile(cacheFile())
	if err != nil {
		return nil, err
	}
	var bridges []Bridge
	if err := json.Unmarshal(data, &bridges); err != nil {
		return nil, err
	}
	return bridges, nil
}

// Client represents a LEAP client connection to a Lutron Bridge
type Client struct {
	addr      string
	tlsConfig *tls.Config
	conn      *tls.Conn
	reader    *bufio.Reader
	mu        sync.Mutex

	// Cert paths for reconnection
	certFile string
	keyFile  string
	caFile   string
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
	Href           string     `json:"href"`
	Name           string     `json:"Name"`
	Nickname       string     `json:"Nickname,omitempty"`
	DeviceType     string     `json:"DeviceType"`
	SerialNumber   int        `json:"SerialNumber,omitempty"`
	ModelNumber    string     `json:"ModelNumber,omitempty"`
	LocalZones     []ZoneLink `json:"LocalZones,omitempty"`
	AssociatedArea struct {
		Href string `json:"href"`
	} `json:"AssociatedArea,omitempty"`
}

type ZoneLink struct {
	Href string `json:"href"`
}

type DeviceResponse struct {
	Devices []Device `json:"Devices"`
}

// Zone represents a Lutron Zone
type Zone struct {
	Href        string `json:"href"`
	Name        string `json:"Name"`
	ControlType string `json:"ControlType,omitempty"`
}

type ZoneResponse struct {
	Zones []Zone `json:"Zones"`
}

type ZoneStatus struct {
	Href  string  `json:"href"`
	Level float64 `json:"Level"`
	Zone  struct {
		Href string `json:"href"`
	} `json:"Zone"`
}

type ZoneStatusResponse struct {
	ZoneStatus   *ZoneStatus  `json:"ZoneStatus,omitempty"`
	ZoneStatuses []ZoneStatus `json:"ZoneStatuses,omitempty"`
}

// CommandBody is the outer wrapper for commands
type CommandBody struct {
	Command Command `json:"Command"`
}

type Command struct {
	CommandType string      `json:"CommandType"`
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
		certFile:  certFile,
		keyFile:   keyFile,
		caFile:    caFile,
	}, nil
}

// Connect opens the TLS connection to the bridge
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		c.conn.Close()
	}

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
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Request sends a JSON request and waits for a response (skipping status updates)
func (c *Client) Request(req Message) (Message, error) {
	// Attempt the request
	resp, err := c.doRequest(req)

	// If we got a connection error, try to reconnect once and retry
	if err != nil && (err == io.EOF || isNetErr(err)) {
		if reconnectErr := c.Connect(); reconnectErr == nil {
			return c.doRequest(req)
		}
	}

	return resp, err
}

func isNetErr(err error) bool {
	if err == nil {
		return false
	}
	// Check for common connection errors
	return true // Simplified for now to retry on any error
}

func (c *Client) doRequest(req Message) (Message, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return Message{}, fmt.Errorf("not connected")
	}

	tag := nextTag()
	req.Header.ClientTag = tag
	
data, err := json.Marshal(req)
	if err != nil {
		return Message{}, err
	}

	if leapLogger != nil {
		leapLogger.Printf("-> %s\n", string(data))
	}
	_, err = c.conn.Write(append(data, '\n'))
	if err != nil {
		return Message{}, err
	}

	for {
		c.conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		line, err := c.reader.ReadBytes('\n')
		if err != nil {
			if leapLogger != nil {
				leapLogger.Printf("Read Error: %v\n", err)
			}
			return Message{}, err
		}

		if leapLogger != nil {
			leapLogger.Printf("<- %s\n", string(line))
		}
		var resp Message
		if err := json.Unmarshal(line, &resp); err != nil {
			continue
		}

		// Check if this is the response to OUR request
		if resp.Header.ClientTag == tag {
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

// GetZones retrieves all zones
func (c *Client) GetZones() ([]Zone, error) {
	req := Message{
		CommuniqueType: "ReadRequest",
		Header: Header{
			Url: "/zone",
		},
	}
	resp, err := c.Request(req)
	if err != nil {
		return nil, err
	}
	var body ZoneResponse
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		return nil, err
	}
	return body.Zones, nil
}

// GetAllZoneStatuses retrieves status for all zones in one call
func (c *Client) GetAllZoneStatuses() ([]ZoneStatus, error) {
	req := Message{
		CommuniqueType: "ReadRequest",
		Header: Header{
			Url: "/zone/status",
		},
	}
	resp, err := c.Request(req)
	if err != nil {
		return nil, err
	}
	var body ZoneStatusResponse
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		return nil, err
	}
	return body.ZoneStatuses, nil
}

// GetZoneStatus retrieves the status for a specific zone
func (c *Client) GetZoneStatus(zoneHref string) (ZoneStatus, error) {
	req := Message{
		CommuniqueType: "ReadRequest",
		Header: Header{
			Url: fmt.Sprintf("%s/status", zoneHref),
		},
	}
	resp, err := c.Request(req)
	if err != nil {
		return ZoneStatus{}, err
	}
	var body ZoneStatusResponse
	if err := json.Unmarshal(resp.Body, &body); err != nil {
		return ZoneStatus{}, err
	}
	if body.ZoneStatus == nil {
		return ZoneStatus{}, fmt.Errorf("no zone status in response")
	}
	return *body.ZoneStatus, nil
}

// SetLevel sets the dimming level for a zone (0-100)
func (c *Client) SetLevel(zoneHref string, level float64) error {
	url := fmt.Sprintf("%s/commandprocessor", zoneHref)
	cmdBody := CommandBody{
		Command: Command{
			CommandType: "GoToLevel",
			Parameter: []Parameter{
				{Type: "Level", Value: level},
			},
		},
	}
	
	body, _ := json.Marshal(cmdBody)
	req := Message{
		CommuniqueType: "CreateRequest",
		Header: Header{
			Url: url,
		},
		Body: body,
	}
	
	resp, err := c.Request(req)
	if err != nil {
		return err
	}
	if resp.CommuniqueType == "ExceptionResponse" {
		return fmt.Errorf("bridge returned exception: %s", string(resp.Body))
	}
	return nil
}

// SetAllLevels sets the dimming level for all dimmable devices sequentially for stability
func (c *Client) SetAllLevels(level float64) error {
	devices, err := c.GetDevices()
	if err != nil {
		return err
	}

	var lastErr error
	for _, d := range devices {
		if len(d.LocalZones) > 0 {
			if err := c.SetLevel(d.LocalZones[0].Href, level); err != nil {
				lastErr = err
			}
			// Small pause to allow bridge to process
			time.Sleep(50 * time.Millisecond)
		}
	}

	return lastErr
}

// SetAreaLevel sets the dimming level for all zones within a specific area sequentially
func (c *Client) SetAreaLevel(areaHref string, level float64) error {
	devices, err := c.GetDevices()
	if err != nil {
		return err
	}

	var lastErr error
	for _, d := range devices {
		if d.AssociatedArea.Href == areaHref && len(d.LocalZones) > 0 {
			if err := c.SetLevel(d.LocalZones[0].Href, level); err != nil {
				lastErr = err
			}
			time.Sleep(50 * time.Millisecond)
		}
	}

	return lastErr
}