package cast

import (
	"net"
	"testing"
)

func TestSelectBestIP(t *testing.T) {
	tests := []struct {
		name     string
		ipv4     []net.IP
		ipv6     []net.IP
		expected string
	}{
		{
			name:     "prefer IPv4 over IPv6",
			ipv4:     []net.IP{net.ParseIP("192.168.1.50")},
			ipv6:     []net.IP{net.ParseIP("2001:db8::1")},
			expected: "192.168.1.50",
		},
		{
			name:     "reject link-local IPv6 when no IPv4",
			ipv4:     nil,
			ipv6:     []net.IP{net.ParseIP("fe80::1ff:fe00:1")},
			expected: "",
		},
		{
			name:     "accept global IPv6 when no IPv4",
			ipv4:     nil,
			ipv6:     []net.IP{net.ParseIP("2001:db8::42")},
			expected: "2001:db8::42",
		},
		{
			name:     "reject loopback IPv4 and IPv6",
			ipv4:     []net.IP{net.ParseIP("127.0.0.1")},
			ipv6:     []net.IP{net.ParseIP("::1")},
			expected: "",
		},
		{
			name:     "empty inputs return empty string",
			ipv4:     nil,
			ipv6:     nil,
			expected: "",
		},
		{
			name:     "skip invalid/loopback IPv4 to valid IPv4",
			ipv4:     []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("192.168.4.15")},
			ipv6:     nil,
			expected: "192.168.4.15",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SelectBestIP(tc.ipv4, tc.ipv6)
			if got != tc.expected {
				t.Errorf("SelectBestIP(%v, %v) = %q; want %q", tc.ipv4, tc.ipv6, got, tc.expected)
			}
		})
	}
}
