package camera

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ghchinoy/homectl/pkg/discovery"
)

func TestIsPrivateIPv4(t *testing.T) {
	tests := []struct {
		ip       string
		expected bool
	}{
		{"192.168.1.1", true},
		{"192.168.2.80", true},
		{"10.0.0.1", true},
		{"10.254.0.15", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"172.32.0.1", false},
		{"8.8.8.8", false},
		{"127.0.0.1", false}, // Loopback handled separately in discovery
		{"fe80::1", false},   // IPv6
	}

	for _, tt := range tests {
		ip := net.ParseIP(tt.ip)
		if ip == nil {
			t.Fatalf("Failed to parse test IP %s", tt.ip)
		}
		// In isPrivateIPv4, 127.0.0.1 returns false
		got := isPrivateIPv4(ip)
		if got != tt.expected {
			t.Errorf("isPrivateIPv4(%s) = %v; want %v", tt.ip, got, tt.expected)
		}
	}
}

func TestScanSubnetRTSP(t *testing.T) {
	provider := &DiscoveryProvider{}
	foundIPs := make(map[string]bool)
	var devices []discovery.Device
	var mu sync.Mutex

	// Mock dialer that succeeds only for 192.168.1.42:554
	mockDialer := func(ctx context.Context, network, address string) (net.Conn, error) {
		if address == "192.168.1.42:554" {
			client, server := net.Pipe()
			_ = server.Close()
			return client, nil
		}
		return nil, errors.New("connection refused")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	provider.scanSubnetRTSP(ctx, "192.168.1.", mockDialer, foundIPs, &devices, &mu)

	if len(devices) != 1 {
		t.Fatalf("scanSubnetRTSP() found %d devices, want 1", len(devices))
	}
	if devices[0].IP != "192.168.1.42" {
		t.Errorf("devices[0].IP = %s, want %s", devices[0].IP, "192.168.1.42")
	}
	if devices[0].Provider != "camera" || devices[0].Type != "Camera" {
		t.Errorf("devices[0] metadata = %+v, want Camera provider and type", devices[0])
	}
	if !foundIPs["192.168.1.42"] {
		t.Errorf("foundIPs[%q] = false, want true", "192.168.1.42")
	}

	// Test deduplication
	provider.scanSubnetRTSP(ctx, "192.168.1.", mockDialer, foundIPs, &devices, &mu)
	if len(devices) != 1 {
		t.Errorf("scanSubnetRTSP() after rerun found %d devices, want 1", len(devices))
	}
}
