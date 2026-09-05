package sonos

import (
	"fmt"
	"io"
	"net"
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
			input: `<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/"><item><dc:title>So What</dc:title><dc:creator>Miles Davis</dc:creator><upnp:album>Kind of Blue</upnp:album><res protocolInfo="http-get:*:audio/flac:*">http://192.168.1.10:8000/track.flac</res></item></DIDL-Lite>`,
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


func TestParseSSDPLocation(t *testing.T) {
	tests := []struct {
		name     string
		response string
		expected string
	}{
		{
			name: "standard Sonos SSDP response",
			response: "HTTP/1.1 200 OK\r\n" +
				"CACHE-CONTROL: max-age = 1800\r\n" +
				"EXT:\r\n" +
				"LOCATION: http://192.168.1.99:1400/xml/device_description.xml\r\n" +
				"SERVER: Linux UPnP/1.0 Sonos/84.2-61240 (ZPS21)\r\n" +
				"ST: urn:schemas-upnp-org:device:ZonePlayer:1\r\n" +
				"USN: uuid:RINCON_000E5800000000001::urn:schemas-upnp-org:device:ZonePlayer:1\r\n\r\n",
			expected: "192.168.1.99",
		},
		{
			name: "LOCATION with path only",
			response: "HTTP/1.1 200 OK\r\n" +
				"LOCATION: http://10.0.0.15/desc.xml\r\n\r\n",
			expected: "10.0.0.15",
		},
		{
			name: "lowercase location header",
			response: "HTTP/1.1 200 OK\r\n" +
				"location: http://192.168.1.55:1400/\r\n\r\n",
			expected: "192.168.1.55",
		},
		{
			name: "no LOCATION header",
			response: "HTTP/1.1 200 OK\r\n" +
				"SERVER: Sonos\r\n\r\n",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSSDPLocation(tt.response)
			if got != tt.expected {
				t.Errorf("parseSSDPLocation() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSelectBestIP(t *testing.T) {
	tests := []struct {
		name     string
		ipv4     []net.IP
		ipv6     []net.IP
		expected string
	}{
		{
			name:     "prefer IPv4 over IPv6",
			ipv4:     []net.IP{net.ParseIP("192.168.1.10")},
			ipv6:     []net.IP{net.ParseIP("2001:db8::1")},
			expected: "192.168.1.10",
		},
		{
			name:     "reject link-local IPv6",
			ipv4:     nil,
			ipv6:     []net.IP{net.ParseIP("fe80::1ff:fe00:1")},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectBestIP(tt.ipv4, tt.ipv6)
			if got != tt.expected {
				t.Errorf("selectBestIP() = %q, want %q", got, tt.expected)
			}
		})
	}
}


func TestParseFavorites(t *testing.T) {
	xmlStr := `<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/" xmlns:r="urn:schemas-rinconnetworks-com:metadata-1-0/" xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/">
  <item id="FV:2/1" parentID="FV:2" restricted="true">
    <dc:title>Morning Jazz</dc:title>
    <upnp:class>object.item.audioItem.audioBroadcast</upnp:class>
    <res protocolInfo="x-rincon-mp3radio:*:*:*">x-sonosapi-stream:s12345?sid=254&amp;flags=8224&amp;sn=0</res>
    <r:resMD>&lt;DIDL-Lite&gt;&lt;item&gt;&lt;dc:title&gt;Morning Jazz Station&lt;/dc:title&gt;&lt;/item&gt;&lt;/DIDL-Lite&gt;</r:resMD>
    <upnp:albumArtURI>/getaa?s=1&amp;u=x-sonosapi-stream</upnp:albumArtURI>
    <r:description>Sonos Radio</r:description>
  </item>
  <item id="FV:2/2" parentID="FV:2" restricted="true">
    <dc:title>Chill Vibes</dc:title>
    <upnp:class>object.container.playlistContainer</upnp:class>
    <res protocolInfo="x-rincon-playlist:*:*:*">x-rincon-cpcontainer:1006206cspotify%3aplaylist%3a37i9dQZF1DX4WYpdgoIcn6?sid=9&amp;flags=0&amp;sn=1</res>
    <r:description>Spotify</r:description>
  </item>
</DIDL-Lite>`

	favs, err := ParseFavorites(xmlStr)
	if err != nil {
		t.Fatalf("ParseFavorites failed: %v", err)
	}
	if len(favs) != 2 {
		t.Fatalf("expected 2 favorites, got %d", len(favs))
	}

	if favs[0].ID != "FV:2/1" || favs[0].Title != "Morning Jazz" || favs[0].Description != "Sonos Radio" {
		t.Errorf("unexpected favorite[0]: %+v", favs[0])
	}
	if !strings.Contains(favs[0].ResourceURI, "x-sonosapi-stream:s12345") {
		t.Errorf("expected ResourceURI to contain stream, got %s", favs[0].ResourceURI)
	}
	if !strings.Contains(favs[0].Metadata, "Morning Jazz Station") {
		t.Errorf("expected Metadata to be unescaped XML, got %s", favs[0].Metadata)
	}

	if favs[1].ID != "FV:2/2" || favs[1].Title != "Chill Vibes" || favs[1].Description != "Spotify" {
		t.Errorf("unexpected favorite[1]: %+v", favs[1])
	}
}

func TestPlayStreamValidation(t *testing.T) {
	client := NewClient("192.168.1.100")

	// Invalid URL scheme (ftp)
	err := client.PlayStream("ftp://example.com/audio.mp3", "FTP Stream")
	if err == nil {
		t.Error("expected error for ftp scheme, got nil")
	}

	// Invalid URL (empty/garbage)
	err = client.PlayStream("not-a-url", "")
	if err == nil {
		t.Error("expected error for non-URL, got nil")
	}
}

func TestParseMusicServices(t *testing.T) {
	xmlStr := `<Services Scheme="1.1">
  <Service Id="9" Name="Spotify" Version="1.1" Uri="https://spotify.sonos.com/smapi" SecureUri="https://spotify.sonos.com/smapi" Capabilities="513"/>
  <Service Id="204" Name="Apple Music" Version="1.1" Uri="https://sonos-music.apple.com/smapi" SecureUri="https://sonos-music.apple.com/smapi"/>
</Services>`

	services, err := ParseMusicServices(xmlStr)
	if err != nil {
		t.Fatalf("ParseMusicServices failed: %v", err)
	}
	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}
	if services[0].ID != "9" || services[0].Name != "Spotify" {
		t.Errorf("unexpected service[0]: %+v", services[0])
	}
	if services[1].ID != "204" || services[1].Name != "Apple Music" {
		t.Errorf("unexpected service[1]: %+v", services[1])
	}
}

func TestResolveDefaultService(t *testing.T) {
	services := []MusicService{
		{ID: "9", Name: "Spotify"},
		{ID: "204", Name: "Apple Music"},
		{ID: "160", Name: "Amazon Music"},
	}

	// 1. Configured default matches Spotify
	s, ok := ResolveDefaultService(services, "Spotify")
	if !ok || s.Name != "Spotify" || !s.IsDefault {
		t.Errorf("expected Spotify as default, got %+v (ok=%v)", s, ok)
	}

	// 2. Case-insensitive match on Apple Music
	s, ok = ResolveDefaultService(services, "apple music")
	if !ok || s.Name != "Apple Music" || !s.IsDefault {
		t.Errorf("expected Apple Music as default, got %+v (ok=%v)", s, ok)
	}

	// 3. Match on service ID "160"
	s, ok = ResolveDefaultService(services, "160")
	if !ok || s.Name != "Amazon Music" || !s.IsDefault {
		t.Errorf("expected Amazon Music matching ID 160, got %+v (ok=%v)", s, ok)
	}

	// 4. Configured default is empty -> fallback to first service
	s, ok = ResolveDefaultService(services, "")
	if !ok || s.Name != "Spotify" || !s.IsDefault {
		t.Errorf("expected first service Spotify as fallback, got %+v (ok=%v)", s, ok)
	}

	// 5. Empty services list returns false
	_, ok = ResolveDefaultService(nil, "Spotify")
	if ok {
		t.Error("expected false for empty services list, got true")
	}
}
