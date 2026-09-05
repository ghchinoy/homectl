// Package miio provides discovery for Xiaomi/Roborock devices via the Mi Home protocol.
package miio

import (
	"context"
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

// Discover sends a handshake packet to the broadcast address to find devices
func Discover(timeout time.Duration) ([]Device, error) {
	// 32 bytes of 0xff is the miio hello/handshake packet
	handshake, _ := hex.DecodeString("21310020ffffffffffffffffffffffffffffffffffffffffffffffffffffffff")

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
			conn.WriteToUDP(handshake, dest)
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

		if n >= 32 && !seen[addr.IP.String()] {
			// bytes 8-11: Device ID
			// bytes 12-15: Timestamp
			deviceID := hex.EncodeToString(buf[8:12])
			devices = append(devices, Device{
				IP:       addr.IP.String(),
				DeviceID: deviceID,
			})
			seen[addr.IP.String()] = true
		}
	}

	return devices, nil
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
