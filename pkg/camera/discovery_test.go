package camera

import (
	"net"
	"testing"
)

func TestIsPrivateIPv4(t *testing.T) {
	tests := []struct {
		ip       string
		expected bool
	}{
		{"192.168.1.1", true},
		{"192.168.4.80", true},
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
