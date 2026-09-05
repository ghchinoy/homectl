// Package sonos provides UPnP/SOAP control and GENA event handling for Sonos speakers.
package sonos

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ghchinoy/homectl/modules/core"
	"github.com/grandcat/zeroconf"
)

// DiscoveryProvider implements core.DiscoveryProvider for Sonos
type DiscoveryProvider struct{}

func (p *DiscoveryProvider) Name() string { return "sonos" }

func (p *DiscoveryProvider) Discover(ctx context.Context) ([]core.Device, error) {
	deadline, ok := ctx.Deadline()
	timeout := 5 * time.Second
	if ok {
		timeout = time.Until(deadline)
	}

	sonosDevices, err := Discover(timeout)
	if err != nil {
		return nil, err
	}

	var devices []core.Device
	for _, s := range sonosDevices {
		devices = append(devices, core.Device{
			ID:       s.RinconID,
			Name:     s.Name,
			IP:       s.IP,
			Provider: "sonos",
			Type:     "Speaker",
			Model:    fmt.Sprintf("%s (%s)", s.ModelName, s.ModelNumber),
		})
	}
	return devices, nil
}

// Device represents a discovered Sonos device
type Device struct {
	Name        string `json:"Name"`
	IP          string `json:"IP"`
	RinconID    string `json:"RinconID"`
	ModelName   string `json:"ModelName"`
	ModelNumber string `json:"ModelNumber"`
}

// selectBestIP selects the most appropriate IP address from mDNS service addresses.
// It prioritizes IPv4 addresses and rejects link-local (fe80::) or loopback IPv6 addresses.
func selectBestIP(ipv4 []net.IP, ipv6 []net.IP) string {
	for _, ip := range ipv4 {
		if ip.To4() != nil && !ip.IsLoopback() && !ip.IsUnspecified() {
			return ip.String()
		}
	}
	for _, ip := range ipv6 {
		if ip != nil && !ip.IsLinkLocalUnicast() && !ip.IsLinkLocalMulticast() && !ip.IsLoopback() && !ip.IsUnspecified() {
			return ip.String()
		}
	}
	return ""
}

// Discover performs mDNS discovery to find Sonos devices
func Discover(timeout time.Duration) ([]Device, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, err
	}

	entries := make(chan *zeroconf.ServiceEntry)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err = resolver.Browse(ctx, "_sonos._tcp", "local.", entries)
	if err != nil {
		return nil, err
	}

	var devices []Device
	foundIPs := make(map[string]bool)

	for entry := range entries {
		ip := selectBestIP(entry.AddrIPv4, entry.AddrIPv6)
		if ip == "" || foundIPs[ip] {
			continue
		}

		name, rincon, modelName, modelNum, err := GetDeviceName(ip)
		if err != nil {
			name = entry.Instance
			if atIdx := strings.Index(name, "@"); atIdx != -1 {
				name = name[atIdx+1:]
			}
		}

		devices = append(devices, Device{
			IP:          ip,
			Name:        name,
			RinconID:    rincon,
			ModelName:   modelName,
			ModelNumber: modelNum,
		})
		foundIPs[ip] = true
	}

	// Fallback: If mDNS found zero devices, query via UPnP SSDP M-SEARCH (e.g. for macOS or segmented LANs)
	if len(devices) == 0 {
		ssdpTimeout := timeout
		if ssdpTimeout > 2*time.Second {
			ssdpTimeout = 2 * time.Second
		}
		if ssdpIPs, err := DiscoverSSDP(ssdpTimeout); err == nil {
			for _, ip := range ssdpIPs {
				if foundIPs[ip] {
					continue
				}
				name, rincon, modelName, modelNum, err := GetDeviceName(ip)
				if err != nil {
					name = fmt.Sprintf("Sonos %s", ip)
				}
				devices = append(devices, Device{
					IP:          ip,
					Name:        name,
					RinconID:    rincon,
					ModelName:   modelName,
					ModelNumber: modelNum,
				})
				foundIPs[ip] = true
			}
		}
	}

	sort.Slice(devices, func(i, j int) bool {
		return devices[i].Name < devices[j].Name
	})

	return devices, nil
}

// parseSSDPLocation extracts the host or IP address from an SSDP response LOCATION header.
func parseSSDPLocation(resp string) string {
	lines := strings.Split(resp, "\r\n")
	for _, line := range lines {
		if idx := strings.Index(line, ":"); idx != -1 {
			header := strings.TrimSpace(line[:idx])
			if strings.EqualFold(header, "LOCATION") {
				val := strings.TrimSpace(line[idx+1:])
				val = strings.TrimPrefix(val, "http://")
				val = strings.TrimPrefix(val, "https://")
				if colonIdx := strings.Index(val, ":"); colonIdx != -1 {
					return val[:colonIdx]
				}
				if slashIdx := strings.Index(val, "/"); slashIdx != -1 {
					return val[:slashIdx]
				}
				return val
			}
		}
	}
	return ""
}

// DiscoverSSDP searches for Sonos devices using UPnP SSDP M-SEARCH (multicast UDP on 239.255.255.250:1900).
func DiscoverSSDP(timeout time.Duration) ([]string, error) {
	ssdpAddr, err := net.ResolveUDPAddr("udp4", "239.255.255.250:1900")
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	msg := "M-SEARCH * HTTP/1.1\r\n" +
		"HOST: 239.255.255.250:1900\r\n" +
		"MAN: \"ssdp:discover\"\r\n" +
		"MX: 2\r\n" +
		"ST: urn:schemas-upnp-org:device:ZonePlayer:1\r\n" +
		"\r\n"

	if _, err := conn.WriteTo([]byte(msg), ssdpAddr); err != nil {
		return nil, err
	}

	_ = conn.SetReadDeadline(time.Now().Add(timeout))

	var ips []string
	seen := make(map[string]bool)
	buf := make([]byte, 4096)

	for {
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			break // timeout or closed
		}

		raw := string(buf[:n])
		ip := parseSSDPLocation(raw)
		if ip == "" {
			if udp, ok := addr.(*net.UDPAddr); ok && udp.IP.To4() != nil {
				ip = udp.IP.String()
			}
		}

		if ip != "" && !seen[ip] {
			parsed := net.ParseIP(ip)
			if parsed != nil && !parsed.IsLoopback() && !parsed.IsUnspecified() {
				seen[ip] = true
				ips = append(ips, ip)
			}
		}
	}

	return ips, nil
}

func cacheFile() string {
	return defaultStorage.Path("sonos_cache.json")
}

// SaveCache merges newly discovered devices with existing ones and persists them
func SaveCache(newDevices []Device) error {
	existing, _ := LoadCache()
	merged := make(map[string]Device)
	for _, d := range existing {
		if d.RinconID != "" {
			merged[d.RinconID] = d
		}
	}
	for _, d := range newDevices {
		if d.RinconID != "" {
			merged[d.RinconID] = d
		}
	}
	var result []Device
	for _, d := range merged {
		result = append(result, d)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return defaultStorage.WriteFile("sonos_cache.json", data, 0644)
}

// LoadCache retrieves previously discovered devices from the local storage
func LoadCache() ([]Device, error) {
	data, err := defaultStorage.ReadFile("sonos_cache.json")
	if err != nil {
		return nil, err
	}
	var devices []Device
	if err := json.Unmarshal(data, &devices); err != nil {
		return nil, err
	}
	return devices, nil
}

func hostPort(addr string, defaultPort int) string {
	if strings.Contains(addr, ":") {
		return addr
	}
	return fmt.Sprintf("%s:%d", addr, defaultPort)
}

// GetDeviceName fetches the device name, Rincon ID, model name and model number from the Sonos XML description
func GetDeviceName(ip string) (string, string, string, string, error) {
	url := fmt.Sprintf("http://%s/xml/device_description.xml", hostPort(ip, 1400))
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", "", "", "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", "", "", err
	}

	var desc struct {
		RoomName    string `xml:"device>roomName"`
		DisplayName string `xml:"device>displayName"`
		UDN         string `xml:"device>UDN"`
		ModelName   string `xml:"device>modelName"`
		ModelNumber string `xml:"device>modelNumber"`
	}
	err = xml.Unmarshal(data, &desc)
	if err != nil {
		return "", "", "", "", err
	}

	rincon := desc.UDN
	if strings.HasPrefix(rincon, "uuid:") {
		rincon = rincon[5:]
	}

	if desc.RoomName != "" {
		return desc.RoomName, rincon, desc.ModelName, desc.ModelNumber, nil
	}
	return desc.DisplayName, rincon, desc.ModelName, desc.ModelNumber, nil
}

// Client represents a Sonos UPnP client
type Client struct {
	ip         string
	httpClient *http.Client
	logger     core.Logger
	storage    core.Storage
}

// Option configures a Sonos Client.
type Option func(*Client)

// WithLogger sets the logger for the Client.
func WithLogger(l core.Logger) Option {
	return func(c *Client) {
		if l != nil {
			c.logger = l
		}
	}
}

// WithHTTPClient sets a custom HTTP client (e.g. for mock tests).
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		if hc != nil {
			c.httpClient = hc
		}
	}
}

// WithStorage sets the storage provider for the Client.
func WithStorage(s core.Storage) Option {
	return func(c *Client) {
		if s != nil {
			c.storage = s
		}
	}
}

func (c *Client) log() core.Logger {
	if c != nil && c.logger != nil {
		return c.logger
	}
	return defaultLogger
}

func (c *Client) store() core.Storage {
	if c != nil && c.storage != nil {
		return c.storage
	}
	return defaultStorage
}

// NewClient creates a new Sonos client for a specific IP with optional configuration
func NewClient(ip string, opts ...Option) *Client {
	c := &Client{
		ip: ip,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger:  defaultLogger,
		storage: defaultStorage,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// SOAPAction executes a SOAP command on the Sonos device
func (c *Client) SOAPAction(controlURL, serviceType, action string, args map[string]string) (io.ReadCloser, error) {
	url := fmt.Sprintf("http://%s%s", hostPort(c.ip, 1400), controlURL)

	body := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
  <s:Body>
    <u:%s xmlns:u="%s">
`, action, serviceType)

	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		var escapedValue bytes.Buffer
		xml.EscapeText(&escapedValue, []byte(args[k]))
		body += fmt.Sprintf("      <%s>%s</%s>\n", k, escapedValue.String(), k)
	}

	body += fmt.Sprintf(`    </u:%s>
  </s:Body>
</s:Envelope>`, action)

	req, err := http.NewRequest("POST", url, bytes.NewBufferString(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	// The SOAPAction header must be quoted according to UPnP spec
	req.Header.Set("SOAPAction", fmt.Sprintf("\"%s#%s\"", serviceType, action))

	c.log().Printf("SOAP OUT: %s#%s to %s\n", serviceType, action, url)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	c.log().Printf("SOAP RESP (%d):\n%s\n", resp.StatusCode, string(b))

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SOAP error (%d): %s", resp.StatusCode, string(b))
	}

	return io.NopCloser(bytes.NewBuffer(b)), nil
}

// GetCoordinatorIP returns the IP address of the coordinator for the group this speaker belongs to
func (c *Client) GetCoordinatorIP() (string, error) {
	state, err := c.GetZoneGroupState()
	if err != nil {
		return c.ip, err
	}

	var myRincon string
	_, rincon, _, _, err := GetDeviceName(c.ip)
	if err == nil {
		myRincon = rincon
	}

	for _, g := range state.Groups {
		isMember := false
		for _, m := range g.Members {
			if m.UUID == myRincon || strings.Contains(m.Location, c.ip) {
				isMember = true
				break
			}
		}
		if isMember {
			for _, m := range g.Members {
				if m.UUID == g.Coordinator {
					loc := m.Location
					if strings.HasPrefix(loc, "http://") {
						loc = loc[7:]
					}
					if idx := strings.Index(loc, ":"); idx != -1 {
						return loc[:idx], nil
					}
					return loc, nil
				}
			}
		}
	}
	return c.ip, nil
}

// SetVolume sets the volume (0-100)
func (c *Client) SetVolume(volume int) error {
	body, err := c.SOAPAction(
		"/MediaRenderer/RenderingControl/Control",
		"urn:schemas-upnp-org:service:RenderingControl:1",
		"SetVolume",
		map[string]string{
			"InstanceID":    "0",
			"Channel":       "Master",
			"DesiredVolume": fmt.Sprintf("%d", volume),
		})
	if err != nil {
		return err
	}
	body.Close()
	return nil
}

// GetVolume retrieves the current volume
func (c *Client) GetVolume() (int, error) {
	body, err := c.SOAPAction(
		"/MediaRenderer/RenderingControl/Control",
		"urn:schemas-upnp-org:service:RenderingControl:1",
		"GetVolume",
		map[string]string{
			"InstanceID": "0",
			"Channel":    "Master",
		})
	if err != nil {
		return 0, err
	}
	defer body.Close()

	data, _ := io.ReadAll(body)
	var resp struct {
		CurrentVolume int `xml:"Body>GetVolumeResponse>CurrentVolume"`
	}
	xml.Unmarshal(data, &resp)
	return resp.CurrentVolume, nil
}

func (c *Client) GetQueueCount() (int, error) {
	body, err := c.SOAPAction(
		"/MediaServer/ContentDirectory/Control",
		"urn:schemas-upnp-org:service:ContentDirectory:1",
		"Browse",
		map[string]string{
			"ObjectID":       "Q:0",
			"BrowseFlag":     "BrowseDirectChildren",
			"Filter":         "*",
			"StartingIndex":  "0",
			"RequestedCount": "1",
			"SortCriteria":   "",
		})
	if err != nil {
		return 0, err
	}
	defer body.Close()
	data, _ := io.ReadAll(body)
	var resp struct {
		TotalMatches int `xml:"Body>BrowseResponse>TotalMatches"`
	}
	xml.Unmarshal(data, &resp)
	return resp.TotalMatches, nil
}

// Play starts playback, routing to coordinator and handling fallbacks
func (c *Client) Play() error {
	coordIP, _ := c.GetCoordinatorIP()
	client := c
	if coordIP != c.ip {
		client = NewClient(coordIP, WithHTTPClient(c.httpClient), WithLogger(c.log()), WithStorage(c.store()))
	}

	// 1. Try playing what's already there
	body, err := client.SOAPAction(
		"/MediaRenderer/AVTransport/Control",
		"urn:schemas-upnp-org:service:AVTransport:1",
		"Play",
		map[string]string{
			"InstanceID": "0",
			"Speed":      "1",
		})

	// 2. If it fails with 701 (Transition Not Available), try to load the queue
	if err != nil && strings.Contains(err.Error(), "701") {
		c.log().Println("Play failed with 701, attempting queue restoration fallback...")
		_, rincon, _, _, nameErr := GetDeviceName(client.ip)
		if nameErr == nil {
			count, _ := client.GetQueueCount()
			if count > 0 {
				queueURI := fmt.Sprintf("x-rincon-queue:%s#0", rincon)
				client.SetAVTransportURI(queueURI, "")
				time.Sleep(500 * time.Millisecond)
			} else {
				// Final fallback: Sonos Radio
				radioURI := "x-sonosapi-radio:sonos:158288?sid=303&flags=0&sn=1"
				client.SetAVTransportURI(radioURI, "")
				time.Sleep(500 * time.Millisecond)
			}
			// Retry Play
			body, err = client.SOAPAction(
				"/MediaRenderer/AVTransport/Control",
				"urn:schemas-upnp-org:service:AVTransport:1",
				"Play",
				map[string]string{
					"InstanceID": "0",
					"Speed":      "1",
				})
		}
	}

	if err != nil {
		return err
	}
	body.Close()
	return nil
}

// Pause pauses playback
func (c *Client) Pause() error {
	coordIP, _ := c.GetCoordinatorIP()
	client := c
	if coordIP != c.ip {
		client = NewClient(coordIP)
	}
	body, err := client.SOAPAction(
		"/MediaRenderer/AVTransport/Control",
		"urn:schemas-upnp-org:service:AVTransport:1",
		"Pause",
		map[string]string{
			"InstanceID": "0",
		})
	if err != nil {
		return err
	}
	body.Close()
	return nil
}

// Stop stops playback
func (c *Client) Stop() error {
	coordIP, _ := c.GetCoordinatorIP()
	client := c
	if coordIP != c.ip {
		client = NewClient(coordIP)
	}
	body, err := client.SOAPAction(
		"/MediaRenderer/AVTransport/Control",
		"urn:schemas-upnp-org:service:AVTransport:1",
		"Stop",
		map[string]string{
			"InstanceID": "0",
		})
	if err != nil {
		return err
	}
	body.Close()
	return nil
}

// Next skips to the next track, ignoring 711 (end of queue)
func (c *Client) Next() error {
	coordIP, _ := c.GetCoordinatorIP()
	client := c
	if coordIP != c.ip {
		client = NewClient(coordIP)
	}
	body, err := client.SOAPAction(
		"/MediaRenderer/AVTransport/Control",
		"urn:schemas-upnp-org:service:AVTransport:1",
		"Next",
		map[string]string{
			"InstanceID": "0",
		})
	if err != nil {
		if strings.Contains(err.Error(), "711") {
			return nil
		}
		return err
	}
	body.Close()
	return nil
}

// Previous skips to the previous track, ignoring 711 (start of queue)
func (c *Client) Previous() error {
	coordIP, _ := c.GetCoordinatorIP()
	client := c
	if coordIP != c.ip {
		client = NewClient(coordIP)
	}
	body, err := client.SOAPAction(
		"/MediaRenderer/AVTransport/Control",
		"urn:schemas-upnp-org:service:AVTransport:1",
		"Previous",
		map[string]string{
			"InstanceID": "0",
		})
	if err != nil {
		if strings.Contains(err.Error(), "711") {
			return nil
		}
		return err
	}
	body.Close()
	return nil
}

// SetAVTransportURI sets the current media URI (e.g. for the queue)
func (c *Client) SetAVTransportURI(uri, metadata string) error {
	body, err := c.SOAPAction(
		"/MediaRenderer/AVTransport/Control",
		"urn:schemas-upnp-org:service:AVTransport:1",
		"SetAVTransportURI",
		map[string]string{
			"InstanceID":         "0",
			"CurrentURI":         uri,
			"CurrentURIMetaData": metadata,
		})
	if err != nil {
		return err
	}
	body.Close()
	return nil
}

// Subscribe registers a callback URL for GENA events from a specific service
func (c *Client) Subscribe(serviceURL, callbackURL string, timeout int) (string, error) {
	url := fmt.Sprintf("http://%s%s", hostPort(c.ip, 1400), serviceURL)
	req, _ := http.NewRequest("SUBSCRIBE", url, nil)
	req.Header.Set("CALLBACK", fmt.Sprintf("<%s>", callbackURL))
	req.Header.Set("NT", "upnp:event")
	req.Header.Set("TIMEOUT", fmt.Sprintf("Second-%d", timeout))

	c.log().Printf("SUBSCRIBE: %s to %s (Callback: %s)\n", serviceURL, url, callbackURL)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	c.log().Printf("SUBSCRIBE RESP (%d) from %s\n", resp.StatusCode, c.ip)

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("subscribe error: %d", resp.StatusCode)
	}
	return resp.Header.Get("SID"), nil
}

type TransportInfo struct {
	CurrentTransportState  string
	CurrentTransportStatus string
	CurrentSpeed           string
}

func (c *Client) GetTransportInfo() (TransportInfo, error) {
	body, err := c.SOAPAction(
		"/MediaRenderer/AVTransport/Control",
		"urn:schemas-upnp-org:service:AVTransport:1",
		"GetTransportInfo",
		map[string]string{
			"InstanceID": "0",
		})
	if err != nil {
		return TransportInfo{}, err
	}
	defer body.Close()

	data, _ := io.ReadAll(body)
	var resp struct {
		CurrentTransportState  string `xml:"Body>GetTransportInfoResponse>CurrentTransportState"`
		CurrentTransportStatus string `xml:"Body>GetTransportInfoResponse>CurrentTransportStatus"`
		CurrentSpeed           string `xml:"Body>GetTransportInfoResponse>CurrentSpeed"`
	}
	xml.Unmarshal(data, &resp)
	return TransportInfo{
		CurrentTransportState:  resp.CurrentTransportState,
		CurrentTransportStatus: resp.CurrentTransportStatus,
		CurrentSpeed:           resp.CurrentSpeed,
	}, nil
}

type PositionInfo struct {
	Track         int
	TrackDuration string
	TrackMetaData string
	TrackURI      string
	RelTime       string
}

type TrackMetadata struct {
	Title         string `xml:"title"`
	Artist        string `xml:"creator"`
	Album         string `xml:"album"`
	StreamContent string `xml:"streamContent"`
	AudioFormat   string `xml:"res"`
	AlbumArtURI   string `xml:"albumArtURI"`
}

type MediaInfo struct {
	NrTracks        int
	CurrentURI      string
	NextURIMetaData string
}

func (c *Client) GetMediaInfo() (MediaInfo, error) {
	body, err := c.SOAPAction(
		"/MediaRenderer/AVTransport/Control",
		"urn:schemas-upnp-org:service:AVTransport:1",
		"GetMediaInfo",
		map[string]string{
			"InstanceID": "0",
		})
	if err != nil {
		return MediaInfo{}, err
	}
	defer body.Close()

	data, _ := io.ReadAll(body)
	var resp struct {
		NrTracks        int    `xml:"Body>GetMediaInfoResponse>NrTracks"`
		CurrentURI      string `xml:"Body>GetMediaInfoResponse>CurrentURI"`
		NextURIMetaData string `xml:"Body>GetMediaInfoResponse>NextURIMetaData"`
	}
	xml.Unmarshal(data, &resp)
	return MediaInfo{
		NrTracks:        resp.NrTracks,
		CurrentURI:      resp.CurrentURI,
		NextURIMetaData: resp.NextURIMetaData,
	}, nil
}

func (c *Client) GetPositionInfo() (PositionInfo, error) {
	body, err := c.SOAPAction(
		"/MediaRenderer/AVTransport/Control",
		"urn:schemas-upnp-org:service:AVTransport:1",
		"GetPositionInfo",
		map[string]string{
			"InstanceID": "0",
		})
	if err != nil {
		return PositionInfo{}, err
	}
	defer body.Close()

	data, _ := io.ReadAll(body)
	var resp struct {
		Track         int    `xml:"Body>GetPositionInfoResponse>Track"`
		TrackDuration string `xml:"Body>GetPositionInfoResponse>TrackDuration"`
		TrackMetaData string `xml:"Body>GetPositionInfoResponse>TrackMetaData"`
		TrackURI      string `xml:"Body>GetPositionInfoResponse>TrackURI"`
		RelTime       string `xml:"Body>GetPositionInfoResponse>RelTime"`
	}
	xml.Unmarshal(data, &resp)
	return PositionInfo{
		Track:         resp.Track,
		TrackDuration: resp.TrackDuration,
		TrackMetaData: resp.TrackMetaData,
		TrackURI:      resp.TrackURI,
		RelTime:       resp.RelTime,
	}, nil
}

func (c *Client) GetZoneGroupAttributes() (string, error) {
	body, err := c.SOAPAction(
		"/ZoneGroupTopology/Control",
		"urn:schemas-upnp-org:service:ZoneGroupTopology:1",
		"GetZoneGroupAttributes",
		nil)
	if err != nil {
		return "", err
	}
	defer body.Close()

	data, _ := io.ReadAll(body)
	var resp struct {
		CurrentZoneGroupName string `xml:"Body>GetZoneGroupAttributesResponse>CurrentZoneGroupName"`
	}
	xml.Unmarshal(data, &resp)
	return resp.CurrentZoneGroupName, nil
}

func (c *Client) GetZoneGroupState() (ZoneGroupState, error) {
	body, err := c.SOAPAction(
		"/ZoneGroupTopology/Control",
		"urn:schemas-upnp-org:service:ZoneGroupTopology:1",
		"GetZoneGroupState",
		nil)
	if err != nil {
		return ZoneGroupState{}, err
	}
	defer body.Close()

	data, _ := io.ReadAll(body)
	var resp struct {
		XML string `xml:"Body>GetZoneGroupStateResponse>ZoneGroupState"`
	}
	xml.Unmarshal(data, &resp)
	var state ZoneGroupState
	xml.Unmarshal([]byte(resp.XML), &state)
	return state, nil
}

type ZoneGroup struct {
	ID          string            `xml:"ID,attr"`
	Coordinator string            `xml:"Coordinator,attr"`
	Members     []ZoneGroupMember `xml:"ZoneGroupMember"`
}

type ZoneGroupMember struct {
	UUID             string `xml:"UUID,attr"`
	Location         string `xml:"Location,attr"`
	RoomName         string `xml:"ZoneName,attr"`
	Invisible        bool   `xml:"Invisible,attr"`
	IsZoneStandAlone bool   `xml:"IsZoneStandAlone,attr"`
}

type ZoneGroupState struct {
	Groups []ZoneGroup `xml:"ZoneGroups>ZoneGroup"`
}

func (c *Client) ParseTrackMetadata(xmlStr string) (TrackMetadata, error) {
	if xmlStr == "" || xmlStr == "NOT_IMPLEMENTED" {
		return TrackMetadata{}, nil
	}
	var meta TrackMetadata
	findTag := func(s, tag string) string {
		start := strings.Index(s, "<"+tag+">")
		if start == -1 {
			start = strings.Index(s, ":"+tag+">")
			if start == -1 {
				return ""
			}
			start += len(tag) + 2
		} else {
			start += len(tag) + 2
		}
		end := strings.Index(s[start:], "</")
		if end == -1 {
			return ""
		}
		return html.UnescapeString(s[start : start+end])
	}
	meta.Title = findTag(xmlStr, "title")
	meta.Artist = findTag(xmlStr, "creator")
	meta.Album = findTag(xmlStr, "album")
	meta.StreamContent = findTag(xmlStr, "streamContent")
	meta.AlbumArtURI = findTag(xmlStr, "albumArtURI")
	resIdx := strings.Index(xmlStr, "<res")
	if resIdx != -1 {
		protoIdx := strings.Index(xmlStr[resIdx:], "protocolInfo=")
		if protoIdx != -1 {
			protoIdx += resIdx + 14
			endProto := strings.Index(xmlStr[protoIdx:], "\"")
			if endProto != -1 {
				meta.AudioFormat = xmlStr[protoIdx : protoIdx+endProto]
			}
		}
	}
	return meta, nil
}


// Favorite represents a pinned Sonos favorite item (playlist, album, radio station, etc.).
type Favorite struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Type        string `json:"type"`
	ResourceURI string `json:"resource_uri"`
	Metadata    string `json:"metadata,omitempty"`
	AlbumArtURI string `json:"album_art_uri,omitempty"`
	Description string `json:"description,omitempty"`
}

// BrowseFavorites lists all pinned Sonos Favorites from the speaker's ContentDirectory (FV:2).
func (c *Client) BrowseFavorites() ([]Favorite, error) {
	body, err := c.SOAPAction(
		"/MediaServer/ContentDirectory/Control",
		"urn:schemas-upnp-org:service:ContentDirectory:1",
		"Browse",
		map[string]string{
			"ObjectID":       "FV:2",
			"BrowseFlag":     "BrowseDirectChildren",
			"Filter":         "*",
			"StartingIndex":  "0",
			"RequestedCount": "100",
			"SortCriteria":   "",
		})
	if err != nil {
		return nil, fmt.Errorf("browse favorites: %w", err)
	}
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result string `xml:"Body>BrowseResponse>Result"`
	}
	if err := xml.Unmarshal(data, &resp); err == nil && resp.Result != "" {
		return ParseFavorites(resp.Result)
	}

	resultXML := extractTagContent(string(data), "Result")
	return ParseFavorites(resultXML)
}

// ParseFavorites parses a DIDL-Lite XML string containing Sonos favorites into Favorite structs.
func ParseFavorites(xmlStr string) ([]Favorite, error) {
	if xmlStr == "" {
		return nil, nil
	}

	var favorites []Favorite
	remaining := xmlStr

	for {
		itemStart := strings.Index(remaining, "<item ")
		isContainer := false
		if itemStart == -1 {
			itemStart = strings.Index(remaining, "<container ")
			isContainer = true
		}
		if itemStart == -1 {
			break
		}

		endTag := "</item>"
		if isContainer {
			endTag = "</container>"
		}

		itemEnd := strings.Index(remaining[itemStart:], endTag)
		if itemEnd == -1 {
			break
		}
		itemEnd += itemStart + len(endTag)
		itemXML := remaining[itemStart:itemEnd]
		remaining = remaining[itemEnd:]

		fav := Favorite{
			ID:          extractAttr(itemXML, "id"),
			Title:       extractTagContent(itemXML, "title"),
			Type:        extractTagContent(itemXML, "class"),
			ResourceURI: extractTagContent(itemXML, "res"),
			Metadata:    extractTagContent(itemXML, "resMD"),
			AlbumArtURI: extractTagContent(itemXML, "albumArtURI"),
			Description: extractTagContent(itemXML, "description"),
		}

		if fav.ID != "" && fav.Title != "" {
			favorites = append(favorites, fav)
		}
	}

	return favorites, nil
}

// isContainerFavorite returns true if the favorite represents a container (playlist, album, etc.)
// that must be enqueued into the playback queue rather than set directly as transport URI.
func isContainerFavorite(fav *Favorite) bool {
	if fav == nil {
		return false
	}
	uri := strings.ToLower(fav.ResourceURI)
	if strings.HasPrefix(uri, "x-rincon-cpcontainer:") ||
		strings.HasPrefix(uri, "x-rincon-playlist:") ||
		strings.HasPrefix(uri, "x-file-cifs:") {
		return true
	}
	typ := strings.ToLower(fav.Type)
	return strings.Contains(typ, "container") ||
		strings.Contains(typ, "playlist") ||
		strings.Contains(typ, "album")
}

// PlayFavorite starts playback of a favorite by ID (or title) with mandatory coordinator resolution.
// For container/playlist favorites (e.g. YouTube Music Liked Music, Spotify playlists), it clears the
// queue, enqueues the container with its stored metadata, points the transport to the queue, and begins playback.
func (c *Client) PlayFavorite(idOrTitle string) error {
	favs, err := c.BrowseFavorites()
	if err != nil {
		return fmt.Errorf("failed to browse favorites: %w", err)
	}

	var match *Favorite
	for i := range favs {
		if favs[i].ID == idOrTitle {
			match = &favs[i]
			break
		}
	}
	if match == nil {
		for i := range favs {
			if strings.EqualFold(favs[i].Title, idOrTitle) {
				match = &favs[i]
				break
			}
		}
	}

	if match == nil {
		return fmt.Errorf("favorite %q not found", idOrTitle)
	}

	// Always resolve group coordinator before setting transport URI & playing
	coordIP, _ := c.GetCoordinatorIP()
	targetClient := c
	if coordIP != "" && coordIP != c.ip {
		targetClient = NewClient(coordIP, WithHTTPClient(c.httpClient), WithLogger(c.log()), WithStorage(c.store()))
	}

	if isContainerFavorite(match) {
		// 1. Clear existing queue (replace semantics)
		_ = targetClient.RemoveAllTracksFromQueue()

		// 2. Enqueue container with stored metadata
		if _, err := targetClient.AddURIToQueue(match.ResourceURI, match.Metadata, false); err != nil {
			return fmt.Errorf("enqueue favorite container %q: %w", match.Title, err)
		}

		// 3. Point transport to the local queue
		var rincon string
		_, r, _, _, nameErr := GetDeviceName(targetClient.ip)
		if nameErr == nil && r != "" {
			rincon = r
		}
		if rincon == "" {
			if cached, err := LoadCache(); err == nil {
				for _, d := range cached {
					if d.IP == targetClient.ip && d.RinconID != "" {
						rincon = d.RinconID
						break
					}
				}
			}
		}

		queueURI := "x-rincon-queue:0#0"
		if rincon != "" {
			queueURI = fmt.Sprintf("x-rincon-queue:%s#0", rincon)
		}

		if err := targetClient.SetAVTransportURI(queueURI, ""); err != nil {
			return fmt.Errorf("set queue transport URI for favorite %q: %w", match.Title, err)
		}

		time.Sleep(300 * time.Millisecond)
		if err := targetClient.Play(); err != nil {
			return fmt.Errorf("play favorite queue %q: %w", match.Title, err)
		}
		return nil
	}

	// Single-item favorite (audio stream, internet radio, or individual track)
	if err := targetClient.SetAVTransportURI(match.ResourceURI, match.Metadata); err != nil {
		return fmt.Errorf("set transport URI for favorite %q: %w", match.Title, err)
	}

	time.Sleep(200 * time.Millisecond)
	if err := targetClient.Play(); err != nil {
		return fmt.Errorf("play favorite %q: %w", match.Title, err)
	}

	return nil
}

// PlayStream validates an audio stream URL (http/https minimal scheme check),
// defaults missing title to the URL host, creates minimal DIDL-Lite stream metadata,
// resolves the group coordinator, sets the transport URI, and starts playback.
func (c *Client) PlayStream(streamURL, title string) error {
	u, err := url.Parse(strings.TrimSpace(streamURL))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("invalid stream URL %q: scheme must be http or https", streamURL)
	}

	if title == "" {
		title = u.Host
		if title == "" {
			title = "Audio Stream"
		}
	}

	metadata := fmt.Sprintf(`<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/" xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/"><item id="-1" parentID="-1" restricted="true"><dc:title>%s</dc:title><upnp:class>object.item.audioItem.audioBroadcast</upnp:class><res protocolInfo="http-get:*:audio/mpeg:*">%s</res></item></DIDL-Lite>`,
		html.EscapeString(title),
		html.EscapeString(streamURL))

	// Mandatory coordinator resolution
	coordIP, _ := c.GetCoordinatorIP()
	targetClient := c
	if coordIP != "" && coordIP != c.ip {
		targetClient = NewClient(coordIP, WithHTTPClient(c.httpClient), WithLogger(c.log()), WithStorage(c.store()))
	}

	if err := targetClient.SetAVTransportURI(streamURL, metadata); err != nil {
		return fmt.Errorf("set stream URI: %w", err)
	}

	time.Sleep(200 * time.Millisecond)
	if err := targetClient.Play(); err != nil {
		return fmt.Errorf("play stream: %w", err)
	}

	return nil
}

// AddURIToQueue adds an audio URI to the queue (optionally as next track) on the group coordinator.
// Returns the newly enqueued track position or queue length.
func (c *Client) AddURIToQueue(uri, metadata string, asNext bool) (int, error) {
	// Mandatory coordinator resolution
	coordIP, _ := c.GetCoordinatorIP()
	targetClient := c
	if coordIP != "" && coordIP != c.ip {
		targetClient = NewClient(coordIP, WithHTTPClient(c.httpClient), WithLogger(c.log()), WithStorage(c.store()))
	}

	asNextVal := "0"
	desiredTrack := "0"
	if asNext {
		asNextVal = "1"
		desiredTrack = "1"
	}

	body, err := targetClient.SOAPAction(
		"/MediaRenderer/AVTransport/Control",
		"urn:schemas-upnp-org:service:AVTransport:1",
		"AddURIToQueue",
		map[string]string{
			"InstanceID":                      "0",
			"EnqueuedURI":                     uri,
			"EnqueuedURIMetaData":             metadata,
			"DesiredFirstTrackNumberEnqueued": desiredTrack,
			"EnqueueAsNext":                   asNextVal,
		})
	if err != nil {
		return 0, fmt.Errorf("add URI to queue: %w", err)
	}
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		return 0, err
	}

	var resp struct {
		FirstTrackNumber int `xml:"Body>AddURIToQueueResponse>FirstTrackNumberEnqueued"`
		NewQueueLength   int `xml:"Body>AddURIToQueueResponse>NewQueueLength"`
	}
	_ = xml.Unmarshal(data, &resp)

	if resp.FirstTrackNumber > 0 {
		return resp.FirstTrackNumber, nil
	}
	if resp.NewQueueLength > 0 {
		return resp.NewQueueLength, nil
	}

	if firstStr := extractTagContent(string(data), "FirstTrackNumberEnqueued"); firstStr != "" {
		if n, err := strconv.Atoi(firstStr); err == nil {
			return n, nil
		}
	}
	if lenStr := extractTagContent(string(data), "NewQueueLength"); lenStr != "" {
		if n, err := strconv.Atoi(lenStr); err == nil {
			return n, nil
		}
	}

	return 1, nil
}

// RemoveAllTracksFromQueue clears all tracks from the speaker's playback queue on the group coordinator.
func (c *Client) RemoveAllTracksFromQueue() error {
	coordIP, _ := c.GetCoordinatorIP()
	targetClient := c
	if coordIP != "" && coordIP != c.ip {
		targetClient = NewClient(coordIP, WithHTTPClient(c.httpClient), WithLogger(c.log()), WithStorage(c.store()))
	}

	body, err := targetClient.SOAPAction(
		"/MediaRenderer/AVTransport/Control",
		"urn:schemas-upnp-org:service:AVTransport:1",
		"RemoveAllTracksFromQueue",
		map[string]string{
			"InstanceID": "0",
		})
	if err != nil {
		return fmt.Errorf("remove all tracks from queue: %w", err)
	}
	body.Close()
	return nil
}

// QueueItem represents a single track entry within the Sonos playback queue.
type QueueItem struct {
	Position    int    `json:"position"`
	TrackID     string `json:"track_id,omitempty"`
	Title       string `json:"title"`
	Artist      string `json:"artist,omitempty"`
	Album       string `json:"album,omitempty"`
	URI         string `json:"uri,omitempty"`
	AlbumArtURI string `json:"album_art_uri,omitempty"`
	Duration    string `json:"duration,omitempty"`
}

// QueueResult represents the paginated result of querying the Sonos playback queue.
type QueueResult struct {
	Items        []QueueItem `json:"items"`
	Returned     int         `json:"returned"`
	TotalMatches int         `json:"total_matches"`
	StartIndex   int         `json:"start_index"`
}

// ParseQueueItems parses a DIDL-Lite XML string containing Sonos queue items into QueueItem structs.
// startIndex is the 0-based offset used for calculating 1-based track positions.
func ParseQueueItems(xmlStr string, startIndex int) []QueueItem {
	if xmlStr == "" {
		return nil
	}

	var items []QueueItem
	remaining := xmlStr

	for {
		itemStart := strings.Index(remaining, "<item ")
		if itemStart == -1 {
			break
		}

		endTag := "</item>"
		itemEnd := strings.Index(remaining[itemStart:], endTag)
		if itemEnd == -1 {
			break
		}
		itemEnd += itemStart + len(endTag)
		itemXML := remaining[itemStart:itemEnd]
		remaining = remaining[itemEnd:]

		artist := extractTagContent(itemXML, "creator")
		if artist == "" {
			artist = extractTagContent(itemXML, "artist")
		}

		item := QueueItem{
			Position:    startIndex + len(items) + 1,
			TrackID:     extractAttr(itemXML, "id"),
			Title:       extractTagContent(itemXML, "title"),
			Artist:      artist,
			Album:       extractTagContent(itemXML, "album"),
			URI:         extractTagContent(itemXML, "res"),
			AlbumArtURI: extractTagContent(itemXML, "albumArtURI"),
			Duration:    extractAttr(itemXML, "duration"),
		}

		if item.Title != "" || item.URI != "" {
			items = append(items, item)
		}
	}

	return items
}

// GetQueue retrieves paginated items from the Sonos playback queue on the group coordinator.
// start is the 0-based starting index (default 0).
// count is the maximum number of items to return (default 100 if <= 0).
func (c *Client) GetQueue(start, count int) (QueueResult, error) {
	if start < 0 {
		start = 0
	}
	if count <= 0 {
		count = 100
	}

	coordIP, _ := c.GetCoordinatorIP()
	targetClient := c
	if coordIP != "" && coordIP != c.ip {
		targetClient = NewClient(coordIP, WithHTTPClient(c.httpClient), WithLogger(c.log()), WithStorage(c.store()))
	}

	body, err := targetClient.SOAPAction(
		"/MediaServer/ContentDirectory/Control",
		"urn:schemas-upnp-org:service:ContentDirectory:1",
		"Browse",
		map[string]string{
			"ObjectID":       "Q:0",
			"BrowseFlag":     "BrowseDirectChildren",
			"Filter":         "*",
			"StartingIndex":  strconv.Itoa(start),
			"RequestedCount": strconv.Itoa(count),
			"SortCriteria":   "",
		})
	if err != nil {
		return QueueResult{}, fmt.Errorf("browse queue: %w", err)
	}
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		return QueueResult{}, err
	}

	var resp struct {
		Result         string `xml:"Body>BrowseResponse>Result"`
		NumberReturned int    `xml:"Body>BrowseResponse>NumberReturned"`
		TotalMatches   int    `xml:"Body>BrowseResponse>TotalMatches"`
	}
	_ = xml.Unmarshal(data, &resp)

	resultXML := resp.Result
	if resultXML == "" {
		resultXML = extractTagContent(string(data), "Result")
	}

	if resp.TotalMatches == 0 {
		if tmStr := extractTagContent(string(data), "TotalMatches"); tmStr != "" {
			if tm, err := strconv.Atoi(tmStr); err == nil {
				resp.TotalMatches = tm
			}
		}
	}

	items := ParseQueueItems(resultXML, start)
	returned := len(items)
	if resp.NumberReturned > 0 && resp.NumberReturned < returned {
		returned = resp.NumberReturned
	}

	totalMatches := resp.TotalMatches
	if totalMatches == 0 && len(items) > 0 {
		totalMatches = len(items)
	}

	return QueueResult{
		Items:        items,
		Returned:     returned,
		TotalMatches: totalMatches,
		StartIndex:   start,
	}, nil
}

// MusicService represents a supported or registered streaming service in the Sonos catalog.
type MusicService struct {
	ID           string `json:"id" xml:"Id,attr"`
	Name         string `json:"name" xml:"Name,attr"`
	Version      string `json:"version,omitempty" xml:"Version,attr"`
	URI          string `json:"uri,omitempty" xml:"Uri,attr"`
	SecureURI    string `json:"secure_uri,omitempty" xml:"SecureUri,attr"`
	Capabilities string `json:"capabilities,omitempty" xml:"Capabilities,attr"`
	IsDefault    bool   `json:"is_default"`
}

// ListMusicServices calls ListAvailableServices on /MusicServices/Control and returns available services.
func (c *Client) ListMusicServices() ([]MusicService, error) {
	body, err := c.SOAPAction(
		"/MusicServices/Control",
		"urn:schemas-upnp-org:service:MusicServices:1",
		"ListAvailableServices",
		nil)
	if err != nil {
		return nil, fmt.Errorf("list music services: %w", err)
	}
	defer body.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}

	var resp struct {
		XML string `xml:"Body>ListAvailableServicesResponse>AvailableServiceDescriptorList"`
	}
	_ = xml.Unmarshal(data, &resp)

	xmlStr := resp.XML
	if xmlStr == "" {
		xmlStr = extractTagContent(string(data), "AvailableServiceDescriptorList")
	}

	return ParseMusicServices(xmlStr)
}

// ParseMusicServices parses an AvailableServiceDescriptorList XML string into MusicService structs.
func ParseMusicServices(xmlStr string) ([]MusicService, error) {
	if xmlStr == "" {
		return nil, nil
	}

	var doc struct {
		Services []MusicService `xml:"Service"`
	}
	if err := xml.Unmarshal([]byte(xmlStr), &doc); err == nil && len(doc.Services) > 0 {
		return doc.Services, nil
	}

	var rootDoc struct {
		Services []MusicService `xml:"Services>Service"`
	}
	if err := xml.Unmarshal([]byte(xmlStr), &rootDoc); err == nil && len(rootDoc.Services) > 0 {
		return rootDoc.Services, nil
	}

	var services []MusicService
	remaining := xmlStr
	for {
		start := strings.Index(remaining, "<Service ")
		if start == -1 {
			break
		}
		end := strings.Index(remaining[start:], "/>")
		if end == -1 {
			end = strings.Index(remaining[start:], "</Service>")
			if end == -1 {
				break
			}
			end += len("</Service>")
		} else {
			end += len("/>")
		}

		svcXML := remaining[start : start+end]
		remaining = remaining[start+end:]

		svc := MusicService{
			ID:           extractAttr(svcXML, "Id"),
			Name:         extractAttr(svcXML, "Name"),
			Version:      extractAttr(svcXML, "Version"),
			URI:          extractAttr(svcXML, "Uri"),
			SecureURI:    extractAttr(svcXML, "SecureUri"),
			Capabilities: extractAttr(svcXML, "Capabilities"),
		}
		if svc.ID != "" && svc.Name != "" {
			services = append(services, svc)
		}
	}

	return services, nil
}

// ResolveDefaultService marks and returns the default music service matching configuredDefault
// (case-insensitive name or ID). If configuredDefault is empty or not matched, it defaults to the
// first service (e.g. Spotify, Apple Music) if available.
func ResolveDefaultService(services []MusicService, configuredDefault string) (MusicService, bool) {
	if len(services) == 0 {
		return MusicService{}, false
	}

	if configuredDefault != "" {
		for i := range services {
			if strings.EqualFold(services[i].Name, configuredDefault) || services[i].ID == configuredDefault {
				services[i].IsDefault = true
				return services[i], true
			}
		}
	}

	services[0].IsDefault = true
	return services[0], true
}

func extractAttr(xmlStr, attr string) string {
	needle := attr + "=\""
	start := strings.Index(xmlStr, needle)
	if start == -1 {
		return ""
	}
	start += len(needle)
	end := strings.Index(xmlStr[start:], "\"")
	if end == -1 {
		return ""
	}
	return html.UnescapeString(xmlStr[start : start+end])
}

func extractTagContent(xmlStr, tag string) string {
	findOpening := func(s, t string) (int, int) {
		idx := strings.Index(s, "<"+t+">")
		if idx != -1 {
			return idx, idx + len(t) + 2
		}
		idx = strings.Index(s, "<"+t+" ")
		if idx != -1 {
			closeBracket := strings.Index(s[idx:], ">")
			if closeBracket != -1 {
				return idx, idx + closeBracket + 1
			}
		}
		idx = strings.Index(s, ":"+t+">")
		if idx != -1 {
			openIdx := strings.LastIndex(s[:idx], "<")
			if openIdx != -1 {
				return openIdx, idx + len(t) + 2
			}
		}
		idx = strings.Index(s, ":"+t+" ")
		if idx != -1 {
			openIdx := strings.LastIndex(s[:idx], "<")
			if openIdx != -1 {
				closeBracket := strings.Index(s[idx:], ">")
				if closeBracket != -1 {
					return openIdx, idx + closeBracket + 1
				}
			}
		}
		return -1, -1
	}

	_, contentStart := findOpening(xmlStr, tag)
	if contentStart == -1 {
		return ""
	}

	content := xmlStr[contentStart:]
	closeTag := "</" + tag + ">"
	endIdx := strings.Index(content, closeTag)
	if endIdx == -1 {
		colonClose := ":" + tag + ">"
		cIdx := strings.Index(content, colonClose)
		if cIdx != -1 {
			openSlash := strings.LastIndex(content[:cIdx], "</")
			if openSlash != -1 {
				endIdx = openSlash
			}
		}
	}

	if endIdx == -1 {
		return ""
	}
	return html.UnescapeString(content[:endIdx])
}
