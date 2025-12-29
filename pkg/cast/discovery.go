package cast

import (
	"context"
	"strings"
	"time"

	"github.com/ghchinoy/homectl/pkg/discovery"
	"github.com/grandcat/zeroconf"
)

// DiscoveryProvider implements discovery.Provider for Google Cast
type DiscoveryProvider struct{}

func (p *DiscoveryProvider) Name() string { return "googlecast" }

func (p *DiscoveryProvider) Discover(ctx context.Context) ([]discovery.Device, error) {
	timeout := 5 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}

	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, err
	}

	entries := make(chan *zeroconf.ServiceEntry)
	browseCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err = resolver.Browse(browseCtx, "_googlecast._tcp", "local.", entries)
	if err != nil {
		return nil, err
	}

	var devices []discovery.Device
	foundIPs := make(map[string]bool)

	for entry := range entries {
		var ip string
		if len(entry.AddrIPv4) > 0 {
			ip = entry.AddrIPv4[0].String()
		} else if len(entry.AddrIPv6) > 0 {
			ip = entry.AddrIPv6[0].String()
		}

		if ip == "" || foundIPs[ip] {
			continue
		}

		// Extract metadata from TXT records
		name := entry.Instance
		model := "Google Cast"
		id := ""

		for _, txt := range entry.Text {
			if strings.HasPrefix(txt, "fn=") {
				name = txt[3:]
			} else if strings.HasPrefix(txt, "md=") {
				model = txt[3:]
			} else if strings.HasPrefix(txt, "id=") {
				id = txt[3:]
			}
		}

		if id == "" {
			id = entry.Instance
		}

		devices = append(devices, discovery.Device{
			ID:       id,
			Name:     name,
			IP:       ip,
			Provider: "googlecast",
			Type:     "Chromecast",
			Model:    model,
		})
		foundIPs[ip] = true
	}

	return devices, nil
}
