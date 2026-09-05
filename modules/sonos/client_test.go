package sonos

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ghchinoy/homectl/modules/core"
)

func TestParseTrackMetadata(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected TrackMetadata
	}{
		{
			name:     "empty metadata",
			input:    "",
			expected: TrackMetadata{},
		},
		{
			name:     "NOT_IMPLEMENTED metadata",
			input:    "NOT_IMPLEMENTED",
			expected: TrackMetadata{},
		},
		{
			name:  "standard DIDL-Lite metadata",
			input: `<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/"><item><dc:title>So What</dc:title><dc:creator>Miles Davis</dc:creator><upnp:album>Kind of Blue</upnp:album><res protocolInfo="http-get:*:audio/flac:*">http://192.168.4.10:8000/track.flac</res></item></DIDL-Lite>`,
			expected: TrackMetadata{
				Title:       "So What",
				Artist:      "Miles Davis",
				Album:       "Kind of Blue",
				AudioFormat: "http-get:*:audio/flac:*",
			},
		},
		{
			name:  "HTML escaped characters in artist and title",
			input: `<item><title>Rock &amp; Roll</title><creator>Led Zeppelin &amp; Friends</creator><album>Led Zeppelin IV &lt;Deluxe&gt;</album></item>`,
			expected: TrackMetadata{
				Title:  "Rock & Roll",
				Artist: "Led Zeppelin & Friends",
				Album:  "Led Zeppelin IV <Deluxe>",
			},
		},
		{
			name:  "Radio stream content",
			input: `<item><title>Live Broadcast</title><streamContent>WNYC: Morning Edition with NPR News</streamContent></item>`,
			expected: TrackMetadata{
				Title:         "Live Broadcast",
				StreamContent: "WNYC: Morning Edition with NPR News",
			},
		},
	}

	c := &Client{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			meta, err := c.ParseTrackMetadata(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if meta.Title != tc.expected.Title {
				t.Errorf("Title: expected %q, got %q", tc.expected.Title, meta.Title)
			}
			if meta.Artist != tc.expected.Artist {
				t.Errorf("Artist: expected %q, got %q", tc.expected.Artist, meta.Artist)
			}
			if meta.Album != tc.expected.Album {
				t.Errorf("Album: expected %q, got %q", tc.expected.Album, meta.Album)
			}
			if meta.StreamContent != tc.expected.StreamContent {
				t.Errorf("StreamContent: expected %q, got %q", tc.expected.StreamContent, meta.StreamContent)
			}
			if meta.AudioFormat != tc.expected.AudioFormat {
				t.Errorf("AudioFormat: expected %q, got %q", tc.expected.AudioFormat, meta.AudioFormat)
			}
		})
	}
}

func TestMockSOAPActions(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		bodyStr := string(bodyBytes)
		soapAction := r.Header.Get("SOAPAction")

		switch {
		case strings.Contains(soapAction, "GetVolume"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetVolumeResponse xmlns:u="urn:schemas-upnp-org:service:RenderingControl:1"><CurrentVolume>35</CurrentVolume></u:GetVolumeResponse></s:Body></s:Envelope>`)

		case strings.Contains(soapAction, "SetVolume"):
			if !strings.Contains(bodyStr, "<DesiredVolume>42</DesiredVolume>") {
				t.Errorf("expected DesiredVolume 42, got %s", bodyStr)
			}
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:SetVolumeResponse xmlns:u="urn:schemas-upnp-org:service:RenderingControl:1"/></s:Body></s:Envelope>`)

		case strings.Contains(soapAction, "GetTransportInfo"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetTransportInfoResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><CurrentTransportState>PLAYING</CurrentTransportState><CurrentTransportStatus>OK</CurrentTransportStatus><CurrentSpeed>1</CurrentSpeed></u:GetTransportInfoResponse></s:Body></s:Envelope>`)

		case strings.Contains(soapAction, "GetPositionInfo"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetPositionInfoResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><TrackDuration>0:03:45</TrackDuration><RelTime>0:01:12</RelTime><TrackURI>http://example.com/audio.mp3</TrackURI><TrackMetaData>&lt;item&gt;&lt;title&gt;Mock Track&lt;/title&gt;&lt;creator&gt;Mock Artist&lt;/creator&gt;&lt;/item&gt;</TrackMetaData></u:GetPositionInfoResponse></s:Body></s:Envelope>`)

		case strings.Contains(soapAction, "Play"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:PlayResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"/></s:Body></s:Envelope>`)

		case strings.Contains(soapAction, "Pause"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:PauseResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"/></s:Body></s:Envelope>`)

		case strings.Contains(soapAction, "Stop"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:StopResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"/></s:Body></s:Envelope>`)

		case strings.Contains(soapAction, "Next"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:NextResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"/></s:Body></s:Envelope>`)

		case strings.Contains(soapAction, "Previous"):
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:PreviousResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"/></s:Body></s:Envelope>`)

		default:
			http.Error(w, "Unknown action", http.StatusInternalServerError)
		}
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	// Extract host and port from server URL (stripping http://)
	serverHost := strings.TrimPrefix(server.URL, "http://")

	client := NewClient(serverHost, WithHTTPClient(server.Client()))

	// 1. Test GetVolume
	vol, err := client.GetVolume()
	if err != nil {
		t.Fatalf("GetVolume failed: %v", err)
	}
	if vol != 35 {
		t.Errorf("expected volume 35, got %d", vol)
	}

	// 2. Test SetVolume
	if err := client.SetVolume(42); err != nil {
		t.Fatalf("SetVolume failed: %v", err)
	}

	// 3. Test GetTransportInfo
	info, err := client.GetTransportInfo()
	if err != nil {
		t.Fatalf("GetTransportInfo failed: %v", err)
	}
	if info.CurrentTransportState != "PLAYING" {
		t.Errorf("expected PLAYING, got %s", info.CurrentTransportState)
	}

	// 4. Test GetPositionInfo
	pos, err := client.GetPositionInfo()
	if err != nil {
		t.Fatalf("GetPositionInfo failed: %v", err)
	}
	if pos.TrackDuration != "0:03:45" {
		t.Errorf("expected 0:03:45, got %s", pos.TrackDuration)
	}
	if pos.RelTime != "0:01:12" {
		t.Errorf("expected 0:01:12, got %s", pos.RelTime)
	}

	// 5. Test control verbs
	if err := client.Pause(); err != nil {
		t.Fatalf("Pause failed: %v", err)
	}
	if err := client.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if err := client.Next(); err != nil {
		t.Fatalf("Next failed: %v", err)
	}
	if err := client.Previous(); err != nil {
		t.Fatalf("Previous failed: %v", err)
	}
}

func TestSOAPErrorHandling(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, `<s:Envelope><s:Body><s:Fault><faultcode>s:Client</faultcode><faultstring>UPnPError</faultstring><detail><UPnPError><errorCode>701</errorCode></UPnPError></detail></s:Fault></s:Body></s:Envelope>`)
	}))
	defer server.Close()

	serverHost := strings.TrimPrefix(server.URL, "http://")
	client := NewClient(serverHost, WithHTTPClient(server.Client()))

	_, err := client.GetVolume()
	if err == nil {
		t.Fatal("expected error from failed SOAP request, got nil")
	}
	if !strings.Contains(err.Error(), "SOAP error (500)") {
		t.Errorf("expected 'SOAP error (500)' in error message, got %v", err)
	}
}

func TestCachePersistenceWithMemoryStorage(t *testing.T) {
	memStorage := core.NewMemoryStorage()
	SetDefaultStorage(memStorage)

	devices := []Device{
		{Name: "Living Room", IP: "192.168.1.101", RinconID: "RINCON_001", ModelName: "Sonos One"},
		{Name: "Kitchen", IP: "192.168.1.102", RinconID: "RINCON_002", ModelName: "Move 2"},
	}

	if err := SaveCache(devices); err != nil {
		t.Fatalf("SaveCache failed: %v", err)
	}

	loaded, err := LoadCache()
	if err != nil {
		t.Fatalf("LoadCache failed: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 devices loaded, got %d", len(loaded))
	}
	if loaded[0].Name != "Kitchen" { // Sorted alphabetically by Name
		t.Errorf("expected first device 'Kitchen', got %s", loaded[0].Name)
	}
	if loaded[1].Name != "Living Room" {
		t.Errorf("expected second device 'Living Room', got %s", loaded[1].Name)
	}
}
