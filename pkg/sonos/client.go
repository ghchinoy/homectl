

package sonos

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

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

type TransportInfo struct {
	CurrentTransportState string
	CurrentTransportStatus string
	CurrentSpeed string
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
	Title  string `xml:"title"`
	Artist string `xml:"creator"`
	Album  string `xml:"album"`
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

// ParseTrackMetadata extracts title, artist, and album from DIDL-Lite XML
func (c *Client) ParseTrackMetadata(xmlStr string) (TrackMetadata, error) {
	if xmlStr == "" || xmlStr == "NOT_IMPLEMENTED" {
		return TrackMetadata{}, nil
	}

	// Sonos embeds XML in XML, often with escaped characters or namespaces
	// We use a simplified approach to find the tags we need
	var meta TrackMetadata
	
	// Helper to find content between tags
	findTag := func(s, tag string) string {
		start := strings.Index(s, "<"+tag+">")
		if start == -1 {
			// Try with namespace
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

	return meta, nil
}
