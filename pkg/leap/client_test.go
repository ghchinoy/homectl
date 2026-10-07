package leap

import (
	"bytes"
	"log"
	"net"
	"testing"
)

func TestSelectBestIP(t *testing.T) {
	tests := []struct {
		name string
		ipv4 []net.IP
		ipv6 []net.IP
		want string
	}{
		{
			name: "prefer IPv4 over IPv6",
			ipv4: []net.IP{net.ParseIP("192.168.1.50")},
			ipv6: []net.IP{net.ParseIP("2001:db8::1")},
			want: "192.168.1.50",
		},
		{
			name: "reject link-local IPv6 when no IPv4",
			ipv4: nil,
			ipv6: []net.IP{net.ParseIP("fe80::1")},
			want: "",
		},
		{
			name: "accept global IPv6 when no IPv4",
			ipv4: nil,
			ipv6: []net.IP{net.ParseIP("2001:db8::1")},
			want: "2001:db8::1",
		},
		{
			name: "reject loopback IPv4 and IPv6",
			ipv4: []net.IP{net.ParseIP("127.0.0.1")},
			ipv6: []net.IP{net.ParseIP("::1")},
			want: "",
		},
		{
			name: "empty inputs return empty string",
			ipv4: nil,
			ipv6: nil,
			want: "",
		},
		{
			name: "fallback to second IPv4 if first is loopback",
			ipv4: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("192.168.1.51")},
			ipv6: nil,
			want: "192.168.1.51",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectBestIP(tt.ipv4, tt.ipv6)
			if got != tt.want {
				t.Errorf("got: %q, want: %q", got, tt.want)
			}
		})
	}
}

func TestSetLogger(t *testing.T) {
	var buf bytes.Buffer
	customLogger := log.New(&buf, "TEST: ", 0)

	SetLogger(customLogger)
	t.Cleanup(func() {
		SetLogger(nil)
	})

	l := getLogger()
	if l != customLogger {
		t.Errorf("got logger %v, want %v", l, customLogger)
	}

	l.Print("hello world")
	got := buf.String()
	want := "TEST: hello world\n"
	if got != want {
		t.Errorf("got: %q, want: %q", got, want)
	}
}
