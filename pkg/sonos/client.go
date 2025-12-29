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
			Timeout: 10 * time.Second,
		},
	}
}

// SOAPAction executes a SOAP command on the Sonos device
func (c *Client) SOAPAction(controlURL, serviceType, action string, args map[string]string) (io.ReadCloser, error) {
	url := fmt.Sprintf("http://%s:1400%s", c.ip, controlURL)
	
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

	if sonosLogger != nil {
		sonosLogger.Printf("SOAP OUT: %s#%s to %s\n", serviceType, action, url)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if sonosLogger != nil {
		sonosLogger.Printf("SOAP RESP (%d):\n%s\n", resp.StatusCode, string(b))
	}

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
			"ObjectID":         "Q:0",
			"BrowseFlag":       "BrowseDirectChildren",
			"Filter":           "*",
			"StartingIndex":    "0",
			"RequestedCount":   "1",
			"SortCriteria":     "",
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
		client = NewClient(coordIP)
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
		if sonosLogger != nil {
			sonosLogger.Println("Play failed with 701, attempting queue restoration fallback...")
		}
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
	url := fmt.Sprintf("http://%s:1400%s", c.ip, serviceURL)
	req, _ := http.NewRequest("SUBSCRIBE", url, nil)
	req.Header.Set("CALLBACK", fmt.Sprintf("<%s>", callbackURL))
	req.Header.Set("NT", "upnp:event")
	req.Header.Set("TIMEOUT", fmt.Sprintf("Second-%d", timeout))

	if sonosLogger != nil {
		sonosLogger.Printf("SUBSCRIBE: %s to %s (Callback: %s)\n", serviceURL, url, callbackURL)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if sonosLogger != nil {
		sonosLogger.Printf("SUBSCRIBE RESP (%d) from %s\n", resp.StatusCode, c.ip)
	}

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
	UUID            string `xml:"UUID,attr"`
	Location        string `xml:"Location,attr"`
	RoomName        string `xml:"ZoneName,attr"`
	Invisible       bool   `xml:"Invisible,attr"`
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