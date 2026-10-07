package miio

import (
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestDiscoveryProvider_Name(t *testing.T) {
	p := &DiscoveryProvider{}
	if got, want := p.Name(), "roborock"; got != want {
		t.Errorf("got: %q, want: %q", got, want)
	}
}

func TestParseHandshakeResponse(t *testing.T) {
	// Construct a synthetic 32-byte packet
	packet := make([]byte, 32)
	// Magic header: 0x21, 0x31, 0x00, 0x20
	packet[0] = 0x21
	packet[1] = 0x31
	packet[2] = 0x00
	packet[3] = 0x20
	// Bytes 4-7: Unknown/Checksum
	// Bytes 8-11: Device ID = 0x0a1b2c3d
	packet[8] = 0x0a
	packet[9] = 0x1b
	packet[10] = 0x2c
	packet[11] = 0x3d
	// Bytes 12-15: Timestamp = 1700000000 (0x6553e100)
	binary.BigEndian.PutUint32(packet[12:16], 1700000000)
	// Bytes 16-31: Token / checksum
	for i := 16; i < 32; i++ {
		packet[i] = 0xff
	}

	t.Run("valid 32-byte packet", func(t *testing.T) {
		dev, ok := parseHandshakeResponse(packet, "192.168.1.150")
		if !ok {
			t.Fatalf("expected packet to parse successfully")
		}
		if got, want := dev.IP, "192.168.1.150"; got != want {
			t.Errorf("IP got: %q, want: %q", got, want)
		}
		if got, want := dev.DeviceID, hex.EncodeToString([]byte{0x0a, 0x1b, 0x2c, 0x3d}); got != want {
			t.Errorf("DeviceID got: %q, want: %q", got, want)
		}
		if got, want := dev.Timestamp, uint32(1700000000); got != want {
			t.Errorf("Timestamp got: %d, want: %d", got, want)
		}
	})

	t.Run("packet larger than 32 bytes parses prefix", func(t *testing.T) {
		largePacket := make([]byte, 64)
		copy(largePacket, packet)
		dev, ok := parseHandshakeResponse(largePacket, "192.168.1.151")
		if !ok {
			t.Fatalf("expected large packet to parse successfully")
		}
		if got, want := dev.DeviceID, "0a1b2c3d"; got != want {
			t.Errorf("DeviceID got: %q, want: %q", got, want)
		}
	})

	t.Run("packet shorter than 32 bytes is rejected", func(t *testing.T) {
		shortPacket := packet[:31]
		_, ok := parseHandshakeResponse(shortPacket, "192.168.1.152")
		if ok {
			t.Errorf("expected truncated packet to be rejected")
		}
	})

	t.Run("empty packet is rejected", func(t *testing.T) {
		_, ok := parseHandshakeResponse(nil, "192.168.1.153")
		if ok {
			t.Errorf("expected empty packet to be rejected")
		}
	})
}
