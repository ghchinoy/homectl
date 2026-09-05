// Package onvif provides WS-Discovery probing for ONVIF-compliant IP cameras.
package onvif

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/ghchinoy/homectl/pkg/discovery"
)

// DiscoveryProvider implements discovery.Provider for ONVIF cameras
type DiscoveryProvider struct{}

func (p *DiscoveryProvider) Name() string { return "onvif" }

func (p *DiscoveryProvider) Discover(ctx context.Context) ([]discovery.Device, error) {
	timeout := 3 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}

	// WS-Discovery Probe XML
	probe := `<?xml version="1.0" encoding="UTF-8"?>
<e:Envelope xmlns:e="http://www.w3.org/2003/05/soap-envelope"
            xmlns:w="http://schemas.xmlsoap.org/ws/2004/08/addressing"
            xmlns:d="http://schemas.xmlsoap.org/ws/2004/08/discovery"
            xmlns:dn="http://www.onvif.org/ver10/network/wsdl">
    <e:Header>
        <w:MessageID>uuid:84ede3de-7dec-11d0-bf15-00a0c9031523</w:MessageID>
        <w:To>urn:schemas-xmlsoap-org:ws:2004:08:discovery</w:To>
        <w:Action>http://schemas.xmlsoap.org/ws/2004/08/discovery/Probe</w:Action>
    </e:Header>
    <e:Body>
        <d:Probe>
            <d:Types>dn:NetworkVideoTransmitter</d:Types>
        </d:Probe>
    </e:Body>
</e:Envelope>`

	addr, err := net.ResolveUDPAddr("udp4", "239.255.255.250:3702")
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	_, err = conn.WriteToUDP([]byte(probe), addr)
	if err != nil {
		return nil, err
	}

	conn.SetReadDeadline(time.Now().Add(timeout))

	var devices []discovery.Device
	foundIPs := make(map[string]bool)

	for {
		buf := make([]byte, 8192)
		n, raddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			break // Timeout
		}

		ip := raddr.IP.String()
		if foundIPs[ip] {
			continue
		}

		resp := string(buf[:n])

		// Very basic XML parsing to extract name/model from Scopes
		name := "ONVIF Camera"
		model := "Unknown"

		// Scopes often look like: onvif://www.onvif.org/name/Front_Door onvif://www.onvif.org/hardware/ADC-V522IR
		scopesIdx := strings.Index(resp, "<d:Scopes>")
		if scopesIdx != -1 {
			scopesEnd := strings.Index(resp[scopesIdx:], "</d:Scopes>")
			if scopesEnd != -1 {
				scopes := resp[scopesIdx+10 : scopesIdx+scopesEnd]
				parts := strings.Fields(scopes)
				for _, p := range parts {
					if strings.Contains(p, "/name/") {
						name = p[strings.LastIndex(p, "/")+1:]
						name = strings.ReplaceAll(name, "_", " ")
					} else if strings.Contains(p, "/hardware/") {
						model = p[strings.LastIndex(p, "/")+1:]
					}
				}
			}
		}

		devices = append(devices, discovery.Device{
			ID:       ip, // Use IP as ID if UDN is too complex to parse here
			Name:     name,
			IP:       ip,
			Provider: "onvif",
			Type:     "Camera",
			Model:    model,
		})
		foundIPs[ip] = true
	}

	return devices, nil
}
