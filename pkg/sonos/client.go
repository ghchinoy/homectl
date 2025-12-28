package sonos

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ghchinoy/control/pkg/config"
	"github.com/ghchinoy/control/pkg/discovery"
	"github.com/grandcat/zeroconf"
)

// DiscoveryProvider implements discovery.Provider for Sonos
type DiscoveryProvider struct{}

func (p *DiscoveryProvider) Name() string { return "sonos" }

func (p *DiscoveryProvider) Discover(ctx context.Context) ([]discovery.Device, error) {
	// Use existing Discover but convert to discovery.Device
	// We need to pass the timeout from context
	deadline, ok := ctx.Deadline()
	timeout := 5 * time.Second
	if ok {
		timeout = time.Until(deadline)
	}

	sonosDevices, err := Discover(timeout)
	if err != nil {
		return nil, err
	}

	var devices []discovery.Device
	for _, s := range sonosDevices {
		devices = append(devices, discovery.Device{
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
		var ip string
		if len(entry.AddrIPv4) > 0 {
			ip = entry.AddrIPv4[0].String()
		} else if len(entry.AddrIPv6) > 0 {
			ip = entry.AddrIPv6[0].String()
		}

		if ip == "" || foundIPs[ip] {
			continue
		}

		// The Name in mDNS might be the serial number (e.g. RINCON_...)
		// Let's try to get a better name from the XML description
		name, rincon, modelName, modelNum, err := GetDeviceName(ip)
		if err != nil {
			// Fallback to mDNS Instance name which often includes "RINCON_SERIAL@Player Name"
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

	// Sort devices by name for a consistent UI
	sort.Slice(devices, func(i, j int) bool {
		return devices[i].Name < devices[j].Name
	})

	return devices, nil
}

func cacheFile() string {
	return config.GetPath("sonos_cache.json")
}

// SaveCache merges newly discovered devices with existing ones and persists them
func SaveCache(newDevices []Device) error {
	// Load existing first
	existing, _ := LoadCache()
	
	// Create a map keyed by RinconID for merging
	merged := make(map[string]Device)
	for _, d := range existing {
		if d.RinconID != "" {
			merged[d.RinconID] = d
		}
	}
	
	// Update with new discovery results
	for _, d := range newDevices {
		if d.RinconID != "" {
			merged[d.RinconID] = d
		}
	}
	
	// Convert back to slice
	var result []Device
	for _, d := range merged {
		result = append(result, d)
	}
	
	// Sort for consistency
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cacheFile(), data, 0644)
}

// LoadCache retrieves previously discovered devices from the local file
func LoadCache() ([]Device, error) {
	data, err := os.ReadFile(cacheFile())
	if err != nil {
		return nil, err
	}
	var devices []Device
	if err := json.Unmarshal(data, &devices); err != nil {
		return nil, err
	}
	return devices, nil
}

// GetDeviceName fetches the device name, Rincon ID, model name and model number from the Sonos XML description
func GetDeviceName(ip string) (string, string, string, string, error) {
	url := fmt.Sprintf("http://%s:1400/xml/device_description.xml", ip)
	resp, err := http.Get(url)
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
}

// NewClient creates a new Sonos client for a specific IP
func NewClient(ip string) *Client {
	return &Client{
		ip: ip,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// SOAPAction executes a SOAP command on the Sonos device
func (c *Client) SOAPAction(controlURL, serviceType, action string, args map[string]interface{}) (io.ReadCloser, error) {
	url := fmt.Sprintf("http://%s:1400%s", c.ip, controlURL)
	
	// Construct the XML body
	body := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:%s xmlns:u="%s">
`, action, serviceType)

	for k, v := range args {
		body += fmt.Sprintf("      <%s>%v</%s>\n", k, v, k)
	}

	body += fmt.Sprintf(`    </u:%s>
  </s:Body>
</s:Envelope>`, action)

	req, err := http.NewRequest("POST", url, bytes.NewBufferString(body))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", fmt.Sprintf("%s#%s", serviceType, action))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("SOAP error (%d): %s", resp.StatusCode, string(b))
	}

	return resp.Body, nil
}

// SetVolume sets the volume (0-100)
func (c *Client) SetVolume(volume int) error {
	body, err := c.SOAPAction(
		"/MediaRenderer/RenderingControl/Control",
		"urn:schemas-upnp-org:service:RenderingControl:1",
		"SetVolume",
		map[string]interface{}{
			"InstanceID":    0,
			"Channel":       "Master",
			"DesiredVolume": volume,
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
		map[string]interface{}{
			"InstanceID": 0,
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

// Play starts playback
func (c *Client) Play() error {
	body, err := c.SOAPAction(
		"/MediaRenderer/AVTransport/Control",
		"urn:schemas-upnp-org:service:AVTransport:1",
		"Play",
		map[string]interface{}{
			"InstanceID": 0,
			"Speed":      1,
		})
	if err != nil {
		return err
	}
	body.Close()
	return nil
}

// Pause pauses playback
func (c *Client) Pause() error {
	body, err := c.SOAPAction(
		"/MediaRenderer/AVTransport/Control",
		"urn:schemas-upnp-org:service:AVTransport:1",
		"Pause",
		map[string]interface{}{
			"InstanceID": 0,
		})
	if err != nil {
		return err
	}
	body.Close()
	return nil
}

// Stop stops playback
func (c *Client) Stop() error {
	body, err := c.SOAPAction(
		"/MediaRenderer/AVTransport/Control",
		"urn:schemas-upnp-org:service:AVTransport:1",
		"Stop",
		map[string]interface{}{
			"InstanceID": 0,
		})
	if err != nil {
		return err
	}
	body.Close()
	return nil
}

// Next skips to the next track
func (c *Client) Next() error {
	body, err := c.SOAPAction(
		"/MediaRenderer/AVTransport/Control",
		"urn:schemas-upnp-org:service:AVTransport:1",
		"Next",
		map[string]interface{}{
			"InstanceID": 0,
		})
	if err != nil {
		return err
	}
	body.Close()
	return nil
}

// Previous skips to the previous track
func (c *Client) Previous() error {
	body, err := c.SOAPAction(
		"/MediaRenderer/AVTransport/Control",
		"urn:schemas-upnp-org:service:AVTransport:1",
		"Previous",
		map[string]interface{}{
			"InstanceID": 0,
		})
	if err != nil {
		return err
	}
	body.Close()
	return nil
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
		map[string]interface{}{
			"InstanceID": 0,
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
	AudioFormat   string `xml:"res"` // ProtocolInfo attribute
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
		map[string]interface{}{
			"InstanceID": 0,
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
		map[string]interface{}{
			"InstanceID": 0,
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

type ZoneGroup struct {
	ID          string            `xml:"ID,attr"`
	Coordinator string            `xml:"Coordinator,attr"`
	Members     []ZoneGroupMember `xml:"ZoneGroupMember"`
}

type ZoneGroupMember struct {
	UUID            string `xml:"UUID,attr"`
	Location        string `xml:"Location,attr"`
	RoomName        string `xml:"ZoneName,attr"`
	Invisible       bool   `xml:"Invisible,attr"`
	IsZoneStandAlone bool   `xml:"IsZoneStandAlone,attr"`
}

type ZoneGroupState struct {
	Groups []ZoneGroup `xml:"ZoneGroups>ZoneGroup"`
}

// Subscribe registers a callback URL for GENA events from a specific service
func (c *Client) Subscribe(serviceURL, callbackURL string, timeout int) (string, error) {
	url := fmt.Sprintf("http://%s:1400%s", c.ip, serviceURL)
	
	req, err := http.NewRequest("SUBSCRIBE", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("CALLBACK", fmt.Sprintf("<%s>", callbackURL))
	req.Header.Set("NT", "upnp:event")
	req.Header.Set("TIMEOUT", fmt.Sprintf("Second-%d", timeout))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("subscribe error: %d", resp.StatusCode)
	}

	return resp.Header.Get("SID"), nil
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
	
	// The response has a ZoneGroupState XML string inside the SOAP response
	var resp struct {
		XML string `xml:"Body>GetZoneGroupStateResponse>ZoneGroupState"`
	}
	xml.Unmarshal(data, &resp)
	
	// Now parse the inner XML
	var state ZoneGroupState
	xml.Unmarshal([]byte(resp.XML), &state)
	return state, nil
}

// ParseTrackMetadata extracts title, artist, album, and more from DIDL-Lite XML
func (c *Client) ParseTrackMetadata(xmlStr string) (TrackMetadata, error) {
	if xmlStr == "" || xmlStr == "NOT_IMPLEMENTED" {
		return TrackMetadata{}, nil
	}

	var meta TrackMetadata
	
	findTag := func(s, tag string) string {
		start := strings.Index(s, "<"+tag+">")
		if start == -1 {
			start = strings.Index(s, ":"+tag+">")
			if start == -1 { return "" }
			start += len(tag) + 2
		} else {
			start += len(tag) + 2
		}
		end := strings.Index(s[start:], "</")
		if end == -1 { return "" }
		return s[start : start+end]
	}

	meta.Title = findTag(xmlStr, "title")
	meta.Artist = findTag(xmlStr, "creator")
	meta.Album = findTag(xmlStr, "album")
	meta.StreamContent = findTag(xmlStr, "streamContent")
	meta.AlbumArtURI = findTag(xmlStr, "albumArtURI")

	// Audio Format is in the protocolInfo attribute of the <res> tag
	resIdx := strings.Index(xmlStr, "<res")
	if resIdx != -1 {
		protoIdx := strings.Index(xmlStr[resIdx:], "protocolInfo=\"")
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
