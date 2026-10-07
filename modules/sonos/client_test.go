package sonos

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
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

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			meta, err := ParseTrackMetadata(tc.input)
			if err != nil {
				t.Fatalf("ParseTrackMetadata() unexpected error: %v", err)
			}
			if meta.Title != tc.expected.Title {
				t.Errorf("Title = %q, want %q", meta.Title, tc.expected.Title)
			}
			if meta.Artist != tc.expected.Artist {
				t.Errorf("Artist = %q, want %q", meta.Artist, tc.expected.Artist)
			}
			if meta.Album != tc.expected.Album {
				t.Errorf("Album = %q, want %q", meta.Album, tc.expected.Album)
			}
			if meta.StreamContent != tc.expected.StreamContent {
				t.Errorf("StreamContent = %q, want %q", meta.StreamContent, tc.expected.StreamContent)
			}
			if meta.AudioFormat != tc.expected.AudioFormat {
				t.Errorf("AudioFormat = %q, want %q", meta.AudioFormat, tc.expected.AudioFormat)
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

func TestIsContainerFavorite(t *testing.T) {
	tests := []struct {
		name     string
		fav      *Favorite
		expected bool
	}{
		{
			name: "YouTube Music Liked Music container",
			fav: &Favorite{
				ResourceURI: "x-rincon-cpcontainer:1006004cALkSOiEkjznR2U-hY1gZPXICcnXWetzSRIrNhw?sid=284&flags=76&sn=2",
				Type:        "object.container.playlistContainer",
			},
			expected: true,
		},
		{
			name: "Spotify playlist container",
			fav: &Favorite{
				ResourceURI: "x-rincon-cpcontainer:1006206cspotify%3aplaylist%3a37i9dQZF1DX4WYpdgoIcn6?sid=9&flags=0&sn=1",
				Type:        "object.container.playlistContainer",
			},
			expected: true,
		},
		{
			name: "Local NAS album container",
			fav: &Favorite{
				ResourceURI: "x-file-cifs://nas/music/PinkFloyd/TheWall",
				Type:        "object.container.album.musicAlbum",
			},
			expected: true,
		},
		{
			name: "Radio stream (not a container)",
			fav: &Favorite{
				ResourceURI: "x-sonosapi-stream:s12345?sid=254&flags=8224&sn=0",
				Type:        "object.item.audioItem.audioBroadcast",
			},
			expected: false,
		},
		{
			name: "Direct MP3 URL (not a container)",
			fav: &Favorite{
				ResourceURI: "http://stream.somafm.com/groovesalad-128-mp3",
				Type:        "object.item.audioItem",
			},
			expected: false,
		},
		{
			name:     "nil favorite",
			fav:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isContainerFavorite(tt.fav)
			if got != tt.expected {
				t.Errorf("isContainerFavorite() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRemoveAllTracksFromQueueMock(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		soapAction := r.Header.Get("SOAPAction")
		if strings.Contains(soapAction, "RemoveAllTracksFromQueue") {
			called = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:RemoveAllTracksFromQueueResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"/></s:Body></s:Envelope>`))
			return
		}
		if strings.Contains(soapAction, "GetZoneGroupState") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetZoneGroupStateResponse xmlns:u="urn:schemas-upnp-org:service:ZoneGroupTopology:1"><ZoneGroupState>&lt;ZoneGroups&gt;&lt;/ZoneGroups&gt;</ZoneGroupState></u:GetZoneGroupStateResponse></s:Body></s:Envelope>`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	client := NewClient(host, WithHTTPClient(server.Client()))
	if err := client.RemoveAllTracksFromQueue(); err != nil {
		t.Fatalf("RemoveAllTracksFromQueue failed: %v", err)
	}
	if !called {
		t.Error("expected RemoveAllTracksFromQueue SOAP action to be called")
	}
}

func TestParseQueueItems(t *testing.T) {
	didl := `<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/" xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/">
		<item id="Q:0/1" parentID="Q:0" restricted="true">
			<dc:title>Track One</dc:title>
			<dc:creator>Artist One</dc:creator>
			<upnp:album>Album One</upnp:album>
			<upnp:albumArtURI>/getaa?u=1</upnp:albumArtURI>
			<res protocolInfo="http-get:*:audio/mp3:*" duration="0:03:45">x-sonos-http:track1.mp3</res>
		</item>
		<item id="Q:0/2" parentID="Q:0" restricted="true">
			<dc:title>Track Two</dc:title>
			<dc:creator>Artist Two</dc:creator>
			<upnp:album>Album Two</upnp:album>
			<res duration="0:04:20">x-file-cifs://nas/track2.flac</res>
		</item>
	</DIDL-Lite>`

	// Test startIndex 0
	items := ParseQueueItems(didl, 0)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Position != 1 {
		t.Errorf("expected position 1, got %d", items[0].Position)
	}
	if items[0].TrackID != "Q:0/1" {
		t.Errorf("expected track id Q:0/1, got %s", items[0].TrackID)
	}
	if items[0].Title != "Track One" {
		t.Errorf("expected title 'Track One', got %s", items[0].Title)
	}
	if items[0].Artist != "Artist One" {
		t.Errorf("expected artist 'Artist One', got %s", items[0].Artist)
	}
	if items[0].Album != "Album One" {
		t.Errorf("expected album 'Album One', got %s", items[0].Album)
	}
	if items[0].Duration != "0:03:45" {
		t.Errorf("expected duration '0:03:45', got %s", items[0].Duration)
	}
	if items[0].URI != "x-sonos-http:track1.mp3" {
		t.Errorf("expected URI 'x-sonos-http:track1.mp3', got %s", items[0].URI)
	}

	if items[1].Position != 2 {
		t.Errorf("expected position 2, got %d", items[1].Position)
	}
	if items[1].Title != "Track Two" {
		t.Errorf("expected title 'Track Two', got %s", items[1].Title)
	}

	// Test startIndex offset
	offsetItems := ParseQueueItems(didl, 10)
	if len(offsetItems) != 2 {
		t.Fatalf("expected 2 items, got %d", len(offsetItems))
	}
	if offsetItems[0].Position != 11 {
		t.Errorf("expected position 11, got %d", offsetItems[0].Position)
	}
	if offsetItems[1].Position != 12 {
		t.Errorf("expected position 12, got %d", offsetItems[1].Position)
	}

	// Test empty XML
	emptyItems := ParseQueueItems("", 0)
	if emptyItems != nil {
		t.Errorf("expected nil for empty XML, got %+v", emptyItems)
	}
}

func TestGetQueueMock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		soapAction := r.Header.Get("SOAPAction")
		if strings.Contains(soapAction, "Browse") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
<s:Body>
<u:BrowseResponse xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1">
<Result>&lt;DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/"&gt;&lt;item id="Q:0/1"&gt;&lt;dc:title&gt;Mock Track&lt;/dc:title&gt;&lt;dc:creator&gt;Mock Artist&lt;/dc:creator&gt;&lt;res duration="0:03:00"&gt;http://audio.mp3&lt;/res&gt;&lt;/item&gt;&lt;/DIDL-Lite&gt;</Result>
<NumberReturned>1</NumberReturned>
<TotalMatches>5</TotalMatches>
<UpdateID>1</UpdateID>
</u:BrowseResponse>
</s:Body>
</s:Envelope>`))
			return
		}
		if strings.Contains(soapAction, "GetZoneGroupState") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetZoneGroupStateResponse xmlns:u="urn:schemas-upnp-org:service:ZoneGroupTopology:1"><ZoneGroupState>&lt;ZoneGroups&gt;&lt;/ZoneGroups&gt;</ZoneGroupState></u:GetZoneGroupStateResponse></s:Body></s:Envelope>`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	client := NewClient(host, WithHTTPClient(server.Client()))
	res, err := client.GetQueue(0, 10)
	if err != nil {
		t.Fatalf("GetQueue failed: %v", err)
	}
	if res.Returned != 1 {
		t.Errorf("expected Returned 1, got %d", res.Returned)
	}
	if res.TotalMatches != 5 {
		t.Errorf("expected TotalMatches 5, got %d", res.TotalMatches)
	}
	if len(res.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(res.Items))
	}
	if res.Items[0].Title != "Mock Track" {
		t.Errorf("expected title 'Mock Track', got %s", res.Items[0].Title)
	}
	if res.Items[0].Position != 1 {
		t.Errorf("expected position 1, got %d", res.Items[0].Position)
	}
}

func TestSeekTrackMock(t *testing.T) {
	var requestedUnit, requestedTarget string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		soapAction := r.Header.Get("SOAPAction")
		if strings.Contains(soapAction, "Seek") {
			body, _ := io.ReadAll(r.Body)
			sBody := string(body)
			requestedUnit = extractTagContent(sBody, "Unit")
			requestedTarget = extractTagContent(sBody, "Target")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:SeekResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"/></s:Body></s:Envelope>`))
			return
		}
		if strings.Contains(soapAction, "GetZoneGroupState") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetZoneGroupStateResponse xmlns:u="urn:schemas-upnp-org:service:ZoneGroupTopology:1"><ZoneGroupState>&lt;ZoneGroups&gt;&lt;/ZoneGroups&gt;</ZoneGroupState></u:GetZoneGroupStateResponse></s:Body></s:Envelope>`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	client := NewClient(host, WithHTTPClient(server.Client()))

	// Valid track seek
	if err := client.SeekTrack(3); err != nil {
		t.Fatalf("SeekTrack failed: %v", err)
	}
	if requestedUnit != "TRACK_NR" {
		t.Errorf("expected Unit TRACK_NR, got %s", requestedUnit)
	}
	if requestedTarget != "3" {
		t.Errorf("expected Target 3, got %s", requestedTarget)
	}

	// Invalid track seek (track < 1)
	if err := client.SeekTrack(0); err == nil {
		t.Error("expected error for track 0, got nil")
	}
}

func TestSeekTimeMock(t *testing.T) {
	var requestedUnit, requestedTarget string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		soapAction := r.Header.Get("SOAPAction")
		if strings.Contains(soapAction, "Seek") {
			body, _ := io.ReadAll(r.Body)
			sBody := string(body)
			requestedUnit = extractTagContent(sBody, "Unit")
			requestedTarget = extractTagContent(sBody, "Target")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:SeekResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"/></s:Body></s:Envelope>`))
			return
		}
		if strings.Contains(soapAction, "GetZoneGroupState") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetZoneGroupStateResponse xmlns:u="urn:schemas-upnp-org:service:ZoneGroupTopology:1"><ZoneGroupState>&lt;ZoneGroups&gt;&lt;/ZoneGroups&gt;</ZoneGroupState></u:GetZoneGroupStateResponse></s:Body></s:Envelope>`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	client := NewClient(host, WithHTTPClient(server.Client()))

	// Seek with MM:SS (normalizes to 0:02:15)
	if err := client.SeekTime("2:15"); err != nil {
		t.Fatalf("SeekTime(2:15) failed: %v", err)
	}
	if requestedUnit != "REL_TIME" {
		t.Errorf("expected Unit REL_TIME, got %s", requestedUnit)
	}
	if requestedTarget != "0:02:15" {
		t.Errorf("expected normalized Target 0:02:15, got %s", requestedTarget)
	}

	// Seek with H:MM:SS
	if err := client.SeekTime("1:05:30"); err != nil {
		t.Fatalf("SeekTime(1:05:30) failed: %v", err)
	}
	if requestedTarget != "1:05:30" {
		t.Errorf("expected Target 1:05:30, got %s", requestedTarget)
	}

	// Invalid seek time format
	if err := client.SeekTime("invalid"); err == nil {
		t.Error("expected error for invalid seek time, got nil")
	}
}

func TestNormalizeSeekTime(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{"0:01:30", "0:01:30", false},
		{"1:45", "0:01:45", false},
		{"01:45", "0:01:45", false},
		{"2:05:10", "2:05:10", false},
		{"0:00:00", "0:00:00", false},
		{"", "", true},
		{"abc", "", true},
		{"1:60", "", true},
		{"1:02:60", "", true},
		{"1:60:00", "", true},
		{"-1:30", "", true},
		{"1:2:3:4", "", true},
	}

	for _, tt := range tests {
		got, err := normalizeSeekTime(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("normalizeSeekTime(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.expected {
			t.Errorf("normalizeSeekTime(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestRemoveTrackRangeFromQueueMock(t *testing.T) {
	var requestedStart, requestedCount string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		soapAction := r.Header.Get("SOAPAction")
		if strings.Contains(soapAction, "RemoveTrackRangeFromQueue") {
			body, _ := io.ReadAll(r.Body)
			sBody := string(body)
			requestedStart = extractTagContent(sBody, "StartingIndex")
			requestedCount = extractTagContent(sBody, "NumberOfTracks")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:RemoveTrackRangeFromQueueResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"/></s:Body></s:Envelope>`))
			return
		}
		if strings.Contains(soapAction, "GetZoneGroupState") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetZoneGroupStateResponse xmlns:u="urn:schemas-upnp-org:service:ZoneGroupTopology:1"><ZoneGroupState>&lt;ZoneGroups&gt;&lt;/ZoneGroups&gt;</ZoneGroupState></u:GetZoneGroupStateResponse></s:Body></s:Envelope>`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	client := NewClient(host, WithHTTPClient(server.Client()))

	// Valid remove
	if err := client.RemoveTrackRangeFromQueue(4, 2); err != nil {
		t.Fatalf("RemoveTrackRangeFromQueue failed: %v", err)
	}
	if requestedStart != "4" {
		t.Errorf("expected StartingIndex 4, got %s", requestedStart)
	}
	if requestedCount != "2" {
		t.Errorf("expected NumberOfTracks 2, got %s", requestedCount)
	}

	// Invalid start (< 1)
	if err := client.RemoveTrackRangeFromQueue(0, 1); err == nil {
		t.Error("expected error for start 0, got nil")
	}
}

func TestReorderTracksInQueueMock(t *testing.T) {
	var requestedStart, requestedCount, requestedInsert string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		soapAction := r.Header.Get("SOAPAction")
		if strings.Contains(soapAction, "ReorderTracksInQueue") {
			body, _ := io.ReadAll(r.Body)
			sBody := string(body)
			requestedStart = extractTagContent(sBody, "StartingIndex")
			requestedCount = extractTagContent(sBody, "NumberOfTracks")
			requestedInsert = extractTagContent(sBody, "InsertBefore")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:ReorderTracksInQueueResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"/></s:Body></s:Envelope>`))
			return
		}
		if strings.Contains(soapAction, "GetZoneGroupState") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetZoneGroupStateResponse xmlns:u="urn:schemas-upnp-org:service:ZoneGroupTopology:1"><ZoneGroupState>&lt;ZoneGroups&gt;&lt;/ZoneGroups&gt;</ZoneGroupState></u:GetZoneGroupStateResponse></s:Body></s:Envelope>`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	client := NewClient(host, WithHTTPClient(server.Client()))

	// Valid reorder
	if err := client.ReorderTracksInQueue(5, 1, 2); err != nil {
		t.Fatalf("ReorderTracksInQueue failed: %v", err)
	}
	if requestedStart != "5" {
		t.Errorf("expected StartingIndex 5, got %s", requestedStart)
	}
	if requestedCount != "1" {
		t.Errorf("expected NumberOfTracks 1, got %s", requestedCount)
	}
	if requestedInsert != "2" {
		t.Errorf("expected InsertBefore 2, got %s", requestedInsert)
	}

	// Invalid parameters
	if err := client.ReorderTracksInQueue(0, 1, 2); err == nil {
		t.Error("expected error for startingIndex 0, got nil")
	}
	if err := client.ReorderTracksInQueue(1, 1, 0); err == nil {
		t.Error("expected error for insertBefore 0, got nil")
	}
}

func TestParseAndBuildPlayMode(t *testing.T) {
	testCases := []struct {
		rawMode    string
		expected   PlayModeSettings
		shuffle    bool
		repeatMode string
		rebuilt    PlayMode
	}{
		{
			rawMode:    "NORMAL",
			expected:   PlayModeSettings{Mode: PlayModeNormal, Shuffle: false, RepeatMode: "off"},
			shuffle:    false,
			repeatMode: "off",
			rebuilt:    PlayModeNormal,
		},
		{
			rawMode:    "REPEAT_ALL",
			expected:   PlayModeSettings{Mode: PlayModeRepeatAll, Shuffle: false, RepeatMode: "all"},
			shuffle:    false,
			repeatMode: "all",
			rebuilt:    PlayModeRepeatAll,
		},
		{
			rawMode:    "REPEAT_ONE",
			expected:   PlayModeSettings{Mode: PlayModeRepeatOne, Shuffle: false, RepeatMode: "one"},
			shuffle:    false,
			repeatMode: "one",
			rebuilt:    PlayModeRepeatOne,
		},
		{
			rawMode:    "SHUFFLE_NOREPEAT",
			expected:   PlayModeSettings{Mode: PlayModeShuffleNoRepeat, Shuffle: true, RepeatMode: "off"},
			shuffle:    true,
			repeatMode: "off",
			rebuilt:    PlayModeShuffleNoRepeat,
		},
		{
			rawMode:    "SHUFFLE",
			expected:   PlayModeSettings{Mode: PlayModeShuffle, Shuffle: true, RepeatMode: "all"},
			shuffle:    true,
			repeatMode: "all",
			rebuilt:    PlayModeShuffle,
		},
		{
			rawMode:    "SHUFFLE_REPEAT_ONE",
			expected:   PlayModeSettings{Mode: PlayModeShuffleRepeatOne, Shuffle: true, RepeatMode: "one"},
			shuffle:    true,
			repeatMode: "one",
			rebuilt:    PlayModeShuffleRepeatOne,
		},
		{
			rawMode:    "UNKNOWN_MODE",
			expected:   PlayModeSettings{Mode: PlayModeNormal, Shuffle: false, RepeatMode: "off"},
			shuffle:    false,
			repeatMode: "none",
			rebuilt:    PlayModeNormal,
		},
	}

	for _, tc := range testCases {
		parsed := ParsePlayMode(tc.rawMode)
		if parsed != tc.expected {
			t.Errorf("ParsePlayMode(%q) = %+v, expected %+v", tc.rawMode, parsed, tc.expected)
		}
		rebuilt := BuildPlayMode(tc.shuffle, tc.repeatMode)
		if rebuilt != tc.rebuilt {
			t.Errorf("BuildPlayMode(%v, %q) = %q, expected %q", tc.shuffle, tc.repeatMode, rebuilt, tc.rebuilt)
		}
	}
}

func TestGetPlayModeMock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		soapAction := r.Header.Get("SOAPAction")
		if strings.Contains(soapAction, "GetTransportSettings") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetTransportSettingsResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><PlayMode>SHUFFLE_NOREPEAT</PlayMode><RecQualityMode>NOT_IMPLEMENTED</RecQualityMode></u:GetTransportSettingsResponse></s:Body></s:Envelope>`))
			return
		}
		if strings.Contains(soapAction, "GetZoneGroupState") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetZoneGroupStateResponse xmlns:u="urn:schemas-upnp-org:service:ZoneGroupTopology:1"><ZoneGroupState>&lt;ZoneGroups&gt;&lt;/ZoneGroups&gt;</ZoneGroupState></u:GetZoneGroupStateResponse></s:Body></s:Envelope>`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	client := NewClient(host, WithHTTPClient(server.Client()))

	settings, err := client.GetPlayMode()
	if err != nil {
		t.Fatalf("GetPlayMode failed: %v", err)
	}
	if settings.Mode != PlayModeShuffleNoRepeat {
		t.Errorf("expected Mode SHUFFLE_NOREPEAT, got %s", settings.Mode)
	}
	if !settings.Shuffle {
		t.Errorf("expected Shuffle true, got false")
	}
	if settings.RepeatMode != "off" {
		t.Errorf("expected RepeatMode off, got %s", settings.RepeatMode)
	}
}

func TestSetShuffleAndRepeatPreservationMock(t *testing.T) {
	currentPlayMode := "REPEAT_ALL"
	var lastSetPlayMode string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		soapAction := r.Header.Get("SOAPAction")
		body, _ := io.ReadAll(r.Body)
		sBody := string(body)

		if strings.Contains(soapAction, "GetTransportSettings") {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetTransportSettingsResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><PlayMode>%s</PlayMode><RecQualityMode>NOT_IMPLEMENTED</RecQualityMode></u:GetTransportSettingsResponse></s:Body></s:Envelope>`, currentPlayMode)
			return
		}
		if strings.Contains(soapAction, "GetMediaInfo") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetMediaInfoResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><CurrentURI>x-rincon-queue:RINCON_12345#0</CurrentURI><NrTracks>10</NrTracks></u:GetMediaInfoResponse></s:Body></s:Envelope>`))
			return
		}
		if strings.Contains(soapAction, "SetPlayMode") {
			lastSetPlayMode = extractTagContent(sBody, "NewPlayMode")
			currentPlayMode = lastSetPlayMode
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:SetPlayModeResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"/></s:Body></s:Envelope>`))
			return
		}
		if strings.Contains(soapAction, "GetZoneGroupState") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetZoneGroupStateResponse xmlns:u="urn:schemas-upnp-org:service:ZoneGroupTopology:1"><ZoneGroupState>&lt;ZoneGroups&gt;&lt;/ZoneGroups&gt;</ZoneGroupState></u:GetZoneGroupStateResponse></s:Body></s:Envelope>`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	client := NewClient(host, WithHTTPClient(server.Client()))

	// 1. Initial state: REPEAT_ALL (shuffle: false, repeat: all).
	// Calling SetShuffle(true) should transition to SHUFFLE (shuffle: true, repeat: all).
	if err := client.SetShuffle(true); err != nil {
		t.Fatalf("SetShuffle(true) failed: %v", err)
	}
	if lastSetPlayMode != "SHUFFLE" {
		t.Errorf("expected NewPlayMode SHUFFLE, got %s", lastSetPlayMode)
	}

	// 2. Current state is SHUFFLE (shuffle: true, repeat: all).
	// Calling SetRepeat("one") should preserve shuffle and set SHUFFLE_REPEAT_ONE.
	if err := client.SetRepeat("one"); err != nil {
		t.Fatalf("SetRepeat(one) failed: %v", err)
	}
	if lastSetPlayMode != "SHUFFLE_REPEAT_ONE" {
		t.Errorf("expected NewPlayMode SHUFFLE_REPEAT_ONE, got %s", lastSetPlayMode)
	}

	// 3. Current state is SHUFFLE_REPEAT_ONE.
	// Calling SetShuffle(false) should preserve repeat: one and set REPEAT_ONE.
	if err := client.SetShuffle(false); err != nil {
		t.Fatalf("SetShuffle(false) failed: %v", err)
	}
	if lastSetPlayMode != "REPEAT_ONE" {
		t.Errorf("expected NewPlayMode REPEAT_ONE, got %s", lastSetPlayMode)
	}

	// 4. Calling SetRepeat("off") should transition to NORMAL.
	if err := client.SetRepeat("off"); err != nil {
		t.Fatalf("SetRepeat(off) failed: %v", err)
	}
	if lastSetPlayMode != "NORMAL" {
		t.Errorf("expected NewPlayMode NORMAL, got %s", lastSetPlayMode)
	}
}

func TestSetPlayModeRadioRejectionMock(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		soapAction := r.Header.Get("SOAPAction")
		if strings.Contains(soapAction, "GetMediaInfo") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetMediaInfoResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><CurrentURI>x-sonosapi-stream:s12345?sid=254&amp;flags=8224&amp;sn=0</CurrentURI><NrTracks>1</NrTracks></u:GetMediaInfoResponse></s:Body></s:Envelope>`))
			return
		}
		if strings.Contains(soapAction, "GetZoneGroupState") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetZoneGroupStateResponse xmlns:u="urn:schemas-upnp-org:service:ZoneGroupTopology:1"><ZoneGroupState>&lt;ZoneGroups&gt;&lt;/ZoneGroups&gt;</ZoneGroupState></u:GetZoneGroupStateResponse></s:Body></s:Envelope>`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	client := NewClient(host, WithHTTPClient(server.Client()))

	err := client.SetPlayMode(PlayModeShuffle)
	if err == nil {
		t.Fatal("expected error setting play mode on live radio stream, got nil")
	}
	if !strings.Contains(err.Error(), "live streams or radio") {
		t.Errorf("expected clear radio rejection error, got: %v", err)
	}
}

func TestCrossfadeModeMock(t *testing.T) {
	crossfadeState := "1"
	var lastSetCrossfade string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		soapAction := r.Header.Get("SOAPAction")
		body, _ := io.ReadAll(r.Body)
		sBody := string(body)

		if strings.Contains(soapAction, "GetCrossfadeMode") {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprintf(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetCrossfadeModeResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><CrossfadeMode>%s</CrossfadeMode></u:GetCrossfadeModeResponse></s:Body></s:Envelope>`, crossfadeState)
			return
		}
		if strings.Contains(soapAction, "SetCrossfadeMode") {
			lastSetCrossfade = extractTagContent(sBody, "CrossfadeMode")
			crossfadeState = lastSetCrossfade
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:SetCrossfadeModeResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"/></s:Body></s:Envelope>`))
			return
		}
		if strings.Contains(soapAction, "GetZoneGroupState") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetZoneGroupStateResponse xmlns:u="urn:schemas-upnp-org:service:ZoneGroupTopology:1"><ZoneGroupState>&lt;ZoneGroups&gt;&lt;/ZoneGroups&gt;</ZoneGroupState></u:GetZoneGroupStateResponse></s:Body></s:Envelope>`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	host := strings.TrimPrefix(server.URL, "http://")
	client := NewClient(host, WithHTTPClient(server.Client()))

	enabled, err := client.GetCrossfadeMode()
	if err != nil {
		t.Fatalf("GetCrossfadeMode failed: %v", err)
	}
	if !enabled {
		t.Errorf("expected initial crossfade true, got false")
	}

	if err := client.SetCrossfadeMode(false); err != nil {
		t.Fatalf("SetCrossfadeMode(false) failed: %v", err)
	}
	if lastSetCrossfade != "0" {
		t.Errorf("expected SetCrossfadeMode sent '0', got %s", lastSetCrossfade)
	}

	enabled, err = client.GetCrossfadeMode()
	if err != nil {
		t.Fatalf("GetCrossfadeMode failed: %v", err)
	}
	if enabled {
		t.Errorf("expected updated crossfade false, got true")
	}
}

func TestPlayModeCoordinatorResolutionMock(t *testing.T) {
	var coordSetPlayMode string

	coordHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		soapAction := r.Header.Get("SOAPAction")
		body, _ := io.ReadAll(r.Body)
		sBody := string(body)

		if strings.Contains(soapAction, "GetTransportSettings") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetTransportSettingsResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><PlayMode>NORMAL</PlayMode><RecQualityMode>NOT_IMPLEMENTED</RecQualityMode></u:GetTransportSettingsResponse></s:Body></s:Envelope>`))
			return
		}
		if strings.Contains(soapAction, "GetMediaInfo") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetMediaInfoResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"><CurrentURI>x-rincon-queue:RINCON_COORD#0</CurrentURI><NrTracks>5</NrTracks></u:GetMediaInfoResponse></s:Body></s:Envelope>`))
			return
		}
		if strings.Contains(soapAction, "SetPlayMode") {
			coordSetPlayMode = extractTagContent(sBody, "NewPlayMode")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:SetPlayModeResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"/></s:Body></s:Envelope>`))
			return
		}
		if strings.Contains(soapAction, "GetZoneGroupState") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetZoneGroupStateResponse xmlns:u="urn:schemas-upnp-org:service:ZoneGroupTopology:1"><ZoneGroupState>&lt;ZoneGroups&gt;&lt;/ZoneGroups&gt;</ZoneGroupState></u:GetZoneGroupStateResponse></s:Body></s:Envelope>`))
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	followerHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		soapAction := r.Header.Get("SOAPAction")
		if strings.Contains(soapAction, "GetZoneGroupState") {
			w.WriteHeader(http.StatusOK)
			// Return topology pointing coordinator to 10.0.0.1
			xmlTopology := `&lt;ZoneGroupState&gt;&lt;ZoneGroups&gt;&lt;ZoneGroup Coordinator="RINCON_COORD" ID="ZG1"&gt;&lt;ZoneGroupMember UUID="RINCON_COORD" Location="http://10.0.0.1:1400/xml/device_description.xml" /&gt;&lt;ZoneGroupMember UUID="RINCON_FOLLOWER" Location="http://10.0.0.2:1400/xml/device_description.xml" /&gt;&lt;/ZoneGroup&gt;&lt;/ZoneGroups&gt;&lt;/ZoneGroupState&gt;`
			_, _ = fmt.Fprintf(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:GetZoneGroupStateResponse xmlns:u="urn:schemas-upnp-org:service:ZoneGroupTopology:1"><ZoneGroupState>%s</ZoneGroupState></u:GetZoneGroupStateResponse></s:Body></s:Envelope>`, xmlTopology)
			return
		}
		// If follower is directly called for SetPlayMode, fail test
		if strings.Contains(soapAction, "SetPlayMode") {
			t.Errorf("SetPlayMode was dispatched directly to follower instead of coordinator!")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mockTransport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		rec := httptest.NewRecorder()
		if strings.HasPrefix(req.URL.Host, "10.0.0.1") {
			coordHandler.ServeHTTP(rec, req)
		} else {
			followerHandler.ServeHTTP(rec, req)
		}
		return rec.Result(), nil
	})

	httpClient := &http.Client{Transport: mockTransport}
	followerClient := NewClient("10.0.0.2", WithHTTPClient(httpClient))

	// Calling SetShuffle on the follower client should resolve to coordinator (10.0.0.1) and execute SetPlayMode there
	if err := followerClient.SetShuffle(true); err != nil {
		t.Fatalf("SetShuffle on follower failed: %v", err)
	}
	if coordSetPlayMode != "SHUFFLE_NOREPEAT" {
		t.Errorf("expected coordinator to receive NewPlayMode SHUFFLE_NOREPEAT, got %s", coordSetPlayMode)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestClassifyGeneration(t *testing.T) {
	tests := []struct {
		name        string
		swGen       string
		modelNumber string
		want        string
	}{
		{name: "swGen_1_explicit", swGen: "1", modelNumber: "S5", want: GenerationS1},
		{name: "swGen_2_explicit", swGen: "2", modelNumber: "S13", want: GenerationS2},
		{name: "swGen_1_on_s2_capable_hardware", swGen: "1", modelNumber: "S1", want: GenerationS1},
		{name: "swGen_missing_s1_only_play5", swGen: "", modelNumber: "S5", want: GenerationS1},
		{name: "swGen_missing_s1_only_bridge", swGen: "", modelNumber: "ZB100", want: GenerationS1},
		{name: "swGen_missing_s1_only_connect", swGen: "", modelNumber: "ZP90", want: GenerationS1},
		{name: "swGen_missing_modern_speaker", swGen: "", modelNumber: "S18", want: GenerationS2},
		{name: "swGen_missing_unknown_model", swGen: "", modelNumber: "UnknownFutureSpeaker", want: GenerationS2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyGeneration(tt.swGen, tt.modelNumber)
			if got != tt.want {
				t.Errorf("ClassifyGeneration(%q, %q) = %q, want %q", tt.swGen, tt.modelNumber, got, tt.want)
			}
		})
	}
}

func TestIsNonRendererModel(t *testing.T) {
	tests := []struct {
		modelNumber string
		want        bool
	}{
		{modelNumber: "ZB100", want: true},
		{modelNumber: "BR100", want: true},
		{modelNumber: "CR100", want: true},
		{modelNumber: "CR200", want: true},
		{modelNumber: "WD100", want: true},
		{modelNumber: "zb100", want: true},
		{modelNumber: "S5", want: false},
		{modelNumber: "S13", want: false},
		{modelNumber: "S18", want: false},
		{modelNumber: "ZP100", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.modelNumber, func(t *testing.T) {
			got := IsNonRendererModel(tt.modelNumber)
			if got != tt.want {
				t.Errorf("IsNonRendererModel(%q) = %v, want %v", tt.modelNumber, got, tt.want)
			}
		})
	}
}

func TestParseDeviceDescription(t *testing.T) {
	tests := []struct {
		name           string
		xmlData        string
		wantName       string
		wantRincon     string
		wantModelName  string
		wantModelNum   string
		wantGeneration string
		wantIsRenderer bool
	}{
		{
			name: "s2_sonos_one_with_avtransport",
			xmlData: `<?xml version="1.0" encoding="utf-8" ?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
  <device>
    <deviceType>urn:schemas-upnp-org:device:ZonePlayer:1</deviceType>
    <roomName>Living Room</roomName>
    <displayName>One</displayName>
    <UDN>uuid:RINCON_000E58A0000E01400</UDN>
    <modelName>Sonos One</modelName>
    <modelNumber>S13</modelNumber>
    <swGen>2</swGen>
    <serviceList>
      <service>
        <serviceType>urn:schemas-upnp-org:service:AVTransport:1</serviceType>
      </service>
      <service>
        <serviceType>urn:schemas-upnp-org:service:RenderingControl:1</serviceType>
      </service>
    </serviceList>
  </device>
</root>`,
			wantName:       "Living Room",
			wantRincon:     "RINCON_000E58A0000E01400",
			wantModelName:  "Sonos One",
			wantModelNum:   "S13",
			wantGeneration: GenerationS2,
			wantIsRenderer: true,
		},
		{
			name: "s1_play5_gen1_with_avtransport",
			xmlData: `<?xml version="1.0" encoding="utf-8" ?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
  <device>
    <deviceType>urn:schemas-upnp-org:device:ZonePlayer:1</deviceType>
    <roomName>Basement</roomName>
    <displayName>Play:5</displayName>
    <UDN>uuid:RINCON_000E58A0000101400</UDN>
    <modelName>Sonos Play:5</modelName>
    <modelNumber>S5</modelNumber>
    <swGen>1</swGen>
    <serviceList>
      <service>
        <serviceType>urn:schemas-upnp-org:service:AVTransport:1</serviceType>
      </service>
      <service>
        <serviceType>urn:schemas-upnp-org:service:RenderingControl:1</serviceType>
      </service>
    </serviceList>
  </device>
</root>`,
			wantName:       "Basement",
			wantRincon:     "RINCON_000E58A0000101400",
			wantModelName:  "Sonos Play:5",
			wantModelNum:   "S5",
			wantGeneration: GenerationS1,
			wantIsRenderer: true,
		},
		{
			name: "s1_bridge_without_avtransport",
			xmlData: `<?xml version="1.0" encoding="utf-8" ?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
  <device>
    <deviceType>urn:schemas-upnp-org:device:ZonePlayer:1</deviceType>
    <roomName>Bridge</roomName>
    <displayName>Bridge</displayName>
    <UDN>uuid:RINCON_000E58A0000B01400</UDN>
    <modelName>Sonos Bridge</modelName>
    <modelNumber>ZB100</modelNumber>
    <swGen>1</swGen>
    <serviceList>
      <service>
        <serviceType>urn:schemas-upnp-org:service:DeviceProperties:1</serviceType>
      </service>
      <service>
        <serviceType>urn:schemas-upnp-org:service:ZoneGroupTopology:1</serviceType>
      </service>
    </serviceList>
  </device>
</root>`,
			wantName:       "Bridge",
			wantRincon:     "RINCON_000E58A0000B01400",
			wantModelName:  "Sonos Bridge",
			wantModelNum:   "ZB100",
			wantGeneration: GenerationS1,
			wantIsRenderer: false,
		},
		{
			name: "legacy_connect_missing_swGen",
			xmlData: `<?xml version="1.0" encoding="utf-8" ?>
<root xmlns="urn:schemas-upnp-org:device-1-0">
  <device>
    <roomName>Den</roomName>
    <UDN>uuid:RINCON_000E58A0000C01400</UDN>
    <modelName>Sonos Connect</modelName>
    <modelNumber>ZP90</modelNumber>
    <serviceList>
      <service>
        <serviceType>urn:schemas-upnp-org:service:AVTransport:1</serviceType>
      </service>
    </serviceList>
  </device>
</root>`,
			wantName:       "Den",
			wantRincon:     "RINCON_000E58A0000C01400",
			wantModelName:  "Sonos Connect",
			wantModelNum:   "ZP90",
			wantGeneration: GenerationS1,
			wantIsRenderer: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			details, err := ParseDeviceDescription([]byte(tt.xmlData))
			if err != nil {
				t.Fatalf("ParseDeviceDescription failed: %v", err)
			}
			if details.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", details.Name, tt.wantName)
			}
			if details.RinconID != tt.wantRincon {
				t.Errorf("RinconID = %q, want %q", details.RinconID, tt.wantRincon)
			}
			if details.ModelName != tt.wantModelName {
				t.Errorf("ModelName = %q, want %q", details.ModelName, tt.wantModelName)
			}
			if details.ModelNumber != tt.wantModelNum {
				t.Errorf("ModelNumber = %q, want %q", details.ModelNumber, tt.wantModelNum)
			}
			if details.Generation != tt.wantGeneration {
				t.Errorf("Generation = %q, want %q", details.Generation, tt.wantGeneration)
			}
			if details.IsRenderer != tt.wantIsRenderer {
				t.Errorf("IsRenderer = %v, want %v", details.IsRenderer, tt.wantIsRenderer)
			}
		})
	}
}

func TestJoinAndLeaveGroupMock(t *testing.T) {
	var capturedAction string
	var capturedURI string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		soapAction := r.Header.Get("SOAPAction")
		bodyBytes, _ := io.ReadAll(r.Body)
		bodyStr := string(bodyBytes)

		if strings.Contains(soapAction, "SetAVTransportURI") {
			capturedAction = "SetAVTransportURI"
			if strings.Contains(bodyStr, "x-rincon:RINCON_123456") {
				capturedURI = "x-rincon:RINCON_123456"
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:SetAVTransportURIResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"/></s:Body></s:Envelope>`))
			return
		}

		if strings.Contains(soapAction, "BecomeCoordinatorOfStandaloneGroup") {
			capturedAction = "BecomeCoordinatorOfStandaloneGroup"
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body><u:BecomeCoordinatorOfStandaloneGroupResponse xmlns:u="urn:schemas-upnp-org:service:AVTransport:1"/></s:Body></s:Envelope>`))
			return
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	u, _ := url.Parse(server.URL)
	client := NewClient(u.Host, WithHTTPClient(server.Client()))

	// Test Join
	if err := client.Join("RINCON_123456"); err != nil {
		t.Fatalf("client.Join failed: %v", err)
	}
	if capturedAction != "SetAVTransportURI" || capturedURI != "x-rincon:RINCON_123456" {
		t.Errorf("unexpected action/URI for Join: action=%q, URI=%q", capturedAction, capturedURI)
	}

	// Test LeaveGroup
	capturedAction = ""
	if err := client.LeaveGroup(); err != nil {
		t.Fatalf("client.LeaveGroup failed: %v", err)
	}
	if capturedAction != "BecomeCoordinatorOfStandaloneGroup" {
		t.Errorf("unexpected action for LeaveGroup: %q", capturedAction)
	}
}
