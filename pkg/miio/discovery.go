// Package miio provides discovery for Xiaomi/Roborock devices via the Mi Home protocol.
package miio

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"net"
	"time"

	"github.com/ghchinoy/homectl/pkg/discovery"
)

// DiscoveryProvider implements discovery.Provider for Roborock/Miio
type DiscoveryProvider struct{}

func (p *DiscoveryProvider) Name() string { return "roborock" }

func (p *DiscoveryProvider) Discover(ctx context.Context) ([]discovery.Device, error) {
	timeout := 2 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}

	miioDevices, err := Discover(timeout)
	if err != nil {
		return nil, err
	}

	var devices []discovery.Device
	for _, d := range miioDevices {
		devices = append(devices, discovery.Device{
			ID:       d.DeviceID,
			Name:     "Roborock Vacuum", // Default until we get metadata
			IP:       d.IP,
			Provider: "roborock",
			Type:     "Vacuum",
		})
	}
	return devices, nil
}

// Device represents a discovered miio device
type Device struct {
	IP        string
	DeviceID  string
	Timestamp uint32
}

// handshakePacket is the 32-byte Mi Home protocol hello/handshake packet.
var handshakePacket = []byte{
	0x21, 0x31, 0x00, 0x20,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	0xff, 0xff, 0xff, 0xff,
}

// Discover sends a handshake packet to the broadcast address to find devices
func Discover(timeout time.Duration) ([]Device, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Find broadcast addresses
	broadcasts, err := getBroadcastAddresses()
	if err != nil {
		broadcasts = []string{"255.255.255.255"}
	}

	for _, addr := range broadcasts {
		dest, err := net.ResolveUDPAddr("udp4", addr+":54321")
		if err == nil {
			_, _ = conn.WriteToUDP(handshakePacket, dest)
		}
	}

	conn.SetReadDeadline(time.Now().Add(timeout))

	var devices []Device
	seen := make(map[string]bool)

	for {
		buf := make([]byte, 64)
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			break // Timeout or other error
		}

		ip := addr.IP.String()
		if !seen[ip] {
			if dev, ok := parseHandshakeResponse(buf[:n], ip); ok {
				devices = append(devices, dev)
				seen[ip] = true
			}
		}
	}

	return devices, nil
}

// parseHandshakeResponse validates and parses a 32-byte Mi Home handshake response packet.
func parseHandshakeResponse(buf []byte, ip string) (Device, bool) {
	if len(buf) < 32 {
		return Device{}, false
	}

	// bytes 8-11: Device ID
	// bytes 12-15: Timestamp (big-endian uint32 epoch)
	deviceID := hex.EncodeToString(buf[8:12])
	ts := binary.BigEndian.Uint32(buf[12:16])

	return Device{
		IP:        ip,
		DeviceID:  deviceID,
		Timestamp: ts,
	}, true
}

func getBroadcastAddresses() ([]string, error) {
	var broadcasts []string
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagBroadcast != 0 {
			addrs, _ := iface.Addrs()
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
					ip := ipnet.IP.To4()
					mask := ipnet.Mask
					broadcast := make(net.IP, len(ip))
					for i := range ip {
						broadcast[i] = ip[i] | ^mask[i]
					}
					broadcasts = append(broadcasts, broadcast.String())
				}
			}
		}
	}
	return broadcasts, nil
}
