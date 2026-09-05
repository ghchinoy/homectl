package sonos

import (
	"testing"
)

func TestParseTrackMetadata(t *testing.T) {
	client := NewClient("127.0.0.1")

	t.Run("empty string", func(t *testing.T) {
		meta, err := client.ParseTrackMetadata("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if meta.Title != "" {
			t.Errorf("expected empty title, got %s", meta.Title)
		}
	})

	t.Run("standard DIDL-Lite metadata", func(t *testing.T) {
		didl := `<DIDL-Lite xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:upnp="urn:schemas-upnp-org:metadata-1-0/upnp/" xmlns="urn:schemas-upnp-org:metadata-1-0/DIDL-Lite/">
			<item id="1" parentID="0" restricted="true">
				<dc:title>Comfortably Numb</dc:title>
				<dc:creator>Pink Floyd</dc:creator>
				<upnp:album>The Wall</upnp:album>
				<upnp:albumArtURI>/getaa?s=1&amp;u=x-sonos-http%3a...%252f</upnp:albumArtURI>
				<res protocolInfo="http-get:*:audio/x-flac:*">http://192.168.4.10:1400/stream</res>
			</item>
		</DIDL-Lite>`

		meta, err := client.ParseTrackMetadata(didl)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if meta.Title != "Comfortably Numb" {
			t.Errorf("expected title 'Comfortably Numb', got '%s'", meta.Title)
		}
		if meta.Artist != "Pink Floyd" {
			t.Errorf("expected artist 'Pink Floyd', got '%s'", meta.Artist)
		}
		if meta.Album != "The Wall" {
			t.Errorf("expected album 'The Wall', got '%s'", meta.Album)
		}
		if meta.AlbumArtURI != "/getaa?s=1&u=x-sonos-http:..." && meta.AlbumArtURI == "" {
			t.Errorf("expected albumArtURI to be populated, got '%s'", meta.AlbumArtURI)
		}
		if meta.AudioFormat != "http-get:*:audio/x-flac:*" {
			t.Errorf("expected format 'http-get:*:audio/x-flac:*', got '%s'", meta.AudioFormat)
		}
	})
}
