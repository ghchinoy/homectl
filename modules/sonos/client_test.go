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
				t.Fatalf("ParseTrackMetadata(%q) unexpected error: %v", tc.name, err)
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
				t.Errorf("SetVolume(42) SOAP body = %s, want <DesiredVolume>42</DesiredVolume>", bodyStr)
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
		t.Errorf("client.GetVolume() = %d, want 35", vol)
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
		t.Errorf("client.GetTransportInfo().CurrentTransportState = %q, want %q", info.CurrentTransportState, "PLAYING")
	}

	// 4. Test GetPositionInfo
	pos, err := client.GetPositionInfo()
	if err != nil {
		t.Fatalf("GetPositionInfo failed: %v", err)
	}
	if pos.TrackDuration != "0:03:45" {
		t.Errorf("client.GetPositionInfo().TrackDuration = %q, want %q", pos.TrackDuration, "0:03:45")
	}
	if pos.RelTime != "0:01:12" {
		t.Errorf("client.GetPositionInfo().RelTime = %q, want %q", pos.RelTime, "0:01:12")
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
		t.Fatal("client.GetVolume() = nil, want SOAP error")
	}
	if !strings.Contains(err.Error(), "SOAP error (500)") {
		t.Errorf("client.GetVolume() error = %v, want substring %q", err, "SOAP error (500)")
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
		t.Fatalf("sonos.LoadCache() len = %d, want 2", len(loaded))
	}
	if loaded[0].Name != "Kitchen" { // Sorted alphabetically by Name
		t.Errorf("sonos.LoadCache()[0].Name = %q, want %q", loaded[0].Name, "Kitchen")
	}
	if loaded[1].Name != "Living Room" {
		t.Errorf("sonos.LoadCache()[1].Name = %q, want %q", loaded[1].Name, "Living Room")
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
				t.Errorf("parseSSDPLocation(%q) = %q, want %q", tt.response, got, tt.expected)
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
				t.Errorf("selectBestIP(%v, %v) = %q, want %q", tt.ipv4, tt.ipv6, got, tt.expected)
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
		t.Fatalf("ParseFavorites(xmlStr) len = %d, want 2", len(favs))
	}

	if favs[0].ID != "FV:2/1" || favs[0].Title != "Morning Jazz" || favs[0].Description != "Sonos Radio" {
		t.Errorf("unexpected favorite[0]: %+v", favs[0])
	}
	if !strings.Contains(favs[0].ResourceURI, "x-sonosapi-stream:s12345") {
		t.Errorf("ParseFavorites(xmlStr)[0].ResourceURI = %q, want stream URI", favs[0].ResourceURI)
	}
	if !strings.Contains(favs[0].Metadata, "Morning Jazz Station") {
		t.Errorf("ParseFavorites(xmlStr)[0].Metadata = %q, want unescaped XML", favs[0].Metadata)
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
		t.Error("PlayStream(\"ftp://example.com/audio.mp3\") = nil, want error")
	}

	// Invalid URL (empty/garbage)
	err = client.PlayStream("not-a-url", "")
	if err == nil {
		t.Error("PlayStream(\"not-a-url\") = nil, want error")
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
		t.Fatalf("ParseMusicServices(xmlStr) len = %d, want 2", len(services))
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
		t.Errorf("ResolveDefaultService(services, \"Spotify\") = (%+v, %v), want Spotify", s, ok)
	}

	// 2. Case-insensitive match on Apple Music
	s, ok = ResolveDefaultService(services, "apple music")
	if !ok || s.Name != "Apple Music" || !s.IsDefault {
		t.Errorf("ResolveDefaultService(services, \"apple music\") = (%+v, %v), want Apple Music", s, ok)
	}

	// 3. Match on service ID "160"
	s, ok = ResolveDefaultService(services, "160")
	if !ok || s.Name != "Amazon Music" || !s.IsDefault {
		t.Errorf("ResolveDefaultService(services, \"160\") = (%+v, %v), want Amazon Music", s, ok)
	}

	// 4. Configured default is empty -> fallback to first service
	s, ok = ResolveDefaultService(services, "")
	if !ok || s.Name != "Spotify" || !s.IsDefault {
		t.Errorf("ResolveDefaultService(services, \"\") = (%+v, %v), want Spotify fallback", s, ok)
	}

	// 5. Empty services list returns false
	_, ok = ResolveDefaultService(nil, "Spotify")
	if ok {
		t.Error("ResolveDefaultService(nil, \"Spotify\") ok = true, want false")
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
				t.Errorf("isContainerFavorite(%+v) = %v, want %v", tt.fav, got, tt.expected)
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
		t.Error("client.RemoveAllTracksFromQueue() SOAP action called = false, want true")
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
		t.Fatalf("ParseQueueItems(didl, 0) len = %d, want 2", len(items))
	}
	if items[0].Position != 1 {
		t.Errorf("ParseQueueItems(didl, 0)[0].Position = %d, want 1", items[0].Position)
	}
	if items[0].TrackID != "Q:0/1" {
		t.Errorf("ParseQueueItems(didl, 0)[0].TrackID = %q, want %q", items[0].TrackID, "Q:0/1")
	}
	if items[0].Title != "Track One" {
		t.Errorf("ParseQueueItems(didl, 0)[0].Title = %q, want %q", items[0].Title, "Track One")
	}
	if items[0].Artist != "Artist One" {
		t.Errorf("ParseQueueItems(didl, 0)[0].Artist = %q, want %q", items[0].Artist, "Artist One")
	}
	if items[0].Album != "Album One" {
		t.Errorf("ParseQueueItems(didl, 0)[0].Album = %q, want %q", items[0].Album, "Album One")
	}
	if items[0].Duration != "0:03:45" {
		t.Errorf("ParseQueueItems(didl, 0)[0].Duration = %q, want %q", items[0].Duration, "0:03:45")
	}
	if items[0].URI != "x-sonos-http:track1.mp3" {
		t.Errorf("ParseQueueItems(didl, 0)[0].URI = %q, want %q", items[0].URI, "x-sonos-http:track1.mp3")
	}

	if items[1].Position != 2 {
		t.Errorf("ParseQueueItems(didl, 0)[1].Position = %d, want 2", items[1].Position)
	}
	if items[1].Title != "Track Two" {
		t.Errorf("ParseQueueItems(didl, 0)[1].Title = %q, want %q", items[1].Title, "Track Two")
	}

	// Test startIndex offset
	offsetItems := ParseQueueItems(didl, 10)
	if len(offsetItems) != 2 {
		t.Fatalf("ParseQueueItems(didl, 10) len = %d, want 2", len(offsetItems))
	}
	if offsetItems[0].Position != 11 {
		t.Errorf("ParseQueueItems(didl, 10)[0].Position = %d, want 11", offsetItems[0].Position)
	}
	if offsetItems[1].Position != 12 {
		t.Errorf("ParseQueueItems(didl, 10)[1].Position = %d, want 12", offsetItems[1].Position)
	}

	// Test empty XML
	emptyItems := ParseQueueItems("", 0)
	if emptyItems != nil {
		t.Errorf("ParseQueueItems(\"\", 0) = %+v, want nil", emptyItems)
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
		t.Errorf("GetQueue(0, 10).Returned = %d, want 1", res.Returned)
	}
	if res.TotalMatches != 5 {
		t.Errorf("GetQueue(0, 10).TotalMatches = %d, want 5", res.TotalMatches)
	}
	if len(res.Items) != 1 {
		t.Fatalf("GetQueue(0, 10) len(Items) = %d, want 1", len(res.Items))
	}
	if res.Items[0].Title != "Mock Track" {
		t.Errorf("GetQueue(0, 10).Items[0].Title = %q, want %q", res.Items[0].Title, "Mock Track")
	}
	if res.Items[0].Position != 1 {
		t.Errorf("GetQueue(0, 10).Items[0].Position = %d, want 1", res.Items[0].Position)
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
		t.Errorf("SeekTrack(3) Unit = %q, want %q", requestedUnit, "TRACK_NR")
	}
	if requestedTarget != "3" {
		t.Errorf("SeekTrack(3) Target = %q, want %q", requestedTarget, "3")
	}

	// Invalid track seek (track < 1)
	if err := client.SeekTrack(0); err == nil {
		t.Error("SeekTrack(0) = nil, want error")
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
		t.Errorf("SeekTime(\"2:15\") Unit = %q, want %q", requestedUnit, "REL_TIME")
	}
	if requestedTarget != "0:02:15" {
		t.Errorf("SeekTime(\"2:15\") Target = %q, want %q", requestedTarget, "0:02:15")
	}

	// Seek with H:MM:SS
	if err := client.SeekTime("1:05:30"); err != nil {
		t.Fatalf("SeekTime(1:05:30) failed: %v", err)
	}
	if requestedTarget != "1:05:30" {
		t.Errorf("SeekTime(\"1:05:30\") Target = %q, want %q", requestedTarget, "1:05:30")
	}

	// Invalid seek time format
	if err := client.SeekTime("invalid"); err == nil {
		t.Error("SeekTime(\"invalid\") = nil, want error")
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
		t.Errorf("RemoveTrackRangeFromQueue(4, 2) StartingIndex = %q, want %q", requestedStart, "4")
	}
	if requestedCount != "2" {
		t.Errorf("RemoveTrackRangeFromQueue(4, 2) NumberOfTracks = %q, want %q", requestedCount, "2")
	}

	// Invalid start (< 1)
	if err := client.RemoveTrackRangeFromQueue(0, 1); err == nil {
		t.Error("RemoveTrackRangeFromQueue(0, 1) = nil, want error")
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
		t.Errorf("ReorderTracksInQueue(5, 1, 2) StartingIndex = %q, want %q", requestedStart, "5")
	}
	if requestedCount != "1" {
		t.Errorf("ReorderTracksInQueue(5, 1, 2) NumberOfTracks = %q, want %q", requestedCount, "1")
	}
	if requestedInsert != "2" {
		t.Errorf("ReorderTracksInQueue(5, 1, 2) InsertBefore = %q, want %q", requestedInsert, "2")
	}

	// Invalid parameters
	if err := client.ReorderTracksInQueue(0, 1, 2); err == nil {
		t.Error("ReorderTracksInQueue(0, 1, 2) = nil, want error")
	}
	if err := client.ReorderTracksInQueue(1, 1, 0); err == nil {
		t.Error("ReorderTracksInQueue(1, 1, 0) = nil, want error")
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
		t.Errorf("client.GetPlayMode().Mode = %q, want %q", settings.Mode, PlayModeShuffleNoRepeat)
	}
	if !settings.Shuffle {
		t.Errorf("client.GetPlayMode().Shuffle = false, want true")
	}
	if settings.RepeatMode != "off" {
		t.Errorf("client.GetPlayMode().RepeatMode = %q, want %q", settings.RepeatMode, "off")
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
		t.Errorf("SetShuffle(true) NewPlayMode = %q, want %q", lastSetPlayMode, "SHUFFLE")
	}

	// 2. Current state is SHUFFLE (shuffle: true, repeat: all).
	// Calling SetRepeat("one") should preserve shuffle and set SHUFFLE_REPEAT_ONE.
	if err := client.SetRepeat("one"); err != nil {
		t.Fatalf("SetRepeat(one) failed: %v", err)
	}
	if lastSetPlayMode != "SHUFFLE_REPEAT_ONE" {
		t.Errorf("SetRepeat(\"one\") NewPlayMode = %q, want %q", lastSetPlayMode, "SHUFFLE_REPEAT_ONE")
	}

	// 3. Current state is SHUFFLE_REPEAT_ONE.
	// Calling SetShuffle(false) should preserve repeat: one and set REPEAT_ONE.
	if err := client.SetShuffle(false); err != nil {
		t.Fatalf("SetShuffle(false) failed: %v", err)
	}
	if lastSetPlayMode != "REPEAT_ONE" {
		t.Errorf("SetShuffle(false) NewPlayMode = %q, want %q", lastSetPlayMode, "REPEAT_ONE")
	}

	// 4. Calling SetRepeat("off") should transition to NORMAL.
	if err := client.SetRepeat("off"); err != nil {
		t.Fatalf("SetRepeat(off) failed: %v", err)
	}
	if lastSetPlayMode != "NORMAL" {
		t.Errorf("SetRepeat(\"off\") NewPlayMode = %q, want %q", lastSetPlayMode, "NORMAL")
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
		t.Fatal("SetPlayMode(PlayModeShuffle) on radio stream = nil, want error")
	}
	if !strings.Contains(err.Error(), "live streams or radio") {
		t.Errorf("SetPlayMode(PlayModeShuffle) error = %v, want radio rejection message", err)
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
		t.Errorf("client.GetCrossfadeMode() = false, want true")
	}

	if err := client.SetCrossfadeMode(false); err != nil {
		t.Fatalf("SetCrossfadeMode(false) failed: %v", err)
	}
	if lastSetCrossfade != "0" {
		t.Errorf("SetCrossfadeMode(false) CrossfadeMode = %q, want %q", lastSetCrossfade, "0")
	}

	enabled, err = client.GetCrossfadeMode()
	if err != nil {
		t.Fatalf("GetCrossfadeMode failed: %v", err)
	}
	if enabled {
		t.Errorf("client.GetCrossfadeMode() = true, want false")
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
		handler := followerHandler
		if strings.HasPrefix(req.URL.Host, "10.0.0.1") {
			handler = coordHandler
		}
		handler.ServeHTTP(rec, req)
		return rec.Result(), nil
	})

	httpClient := &http.Client{Transport: mockTransport}
	followerClient := NewClient("10.0.0.2", WithHTTPClient(httpClient))

	// Calling SetShuffle on the follower client should resolve to coordinator (10.0.0.1) and execute SetPlayMode there
	if err := followerClient.SetShuffle(true); err != nil {
		t.Fatalf("SetShuffle on follower failed: %v", err)
	}
	if coordSetPlayMode != "SHUFFLE_NOREPEAT" {
		t.Errorf("SetShuffle(true) coordinator NewPlayMode = %q, want %q", coordSetPlayMode, "SHUFFLE_NOREPEAT")
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
