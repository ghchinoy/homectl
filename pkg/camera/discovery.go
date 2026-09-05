// Package camera provides discovery and RTSP stream handling for local security cameras.
package camera

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/ghchinoy/homectl/pkg/discovery"
	"github.com/grandcat/zeroconf"
)

// DiscoveryProvider implements discovery.Provider for RTSP cameras
type DiscoveryProvider struct{}

func (p *DiscoveryProvider) Name() string { return "camera" }

func isPrivateIPv4(ip net.IP) bool {
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}
	return ip4[0] == 10 ||
		(ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31) ||
		(ip4[0] == 192 && ip4[1] == 168)
}

func (p *DiscoveryProvider) Discover(ctx context.Context) ([]discovery.Device, error) {
	var devices []discovery.Device
	foundIPs := make(map[string]bool)
	var mu sync.Mutex

	timeout := 2 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}

	// 1. mDNS Discovery for common camera types
	resolver, err := zeroconf.NewResolver(nil)
	if err == nil {
		services := []string{"_rtsp._tcp", "_axis-video._tcp", "_http._tcp"}
		var mdnsWg sync.WaitGroup
		browseCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		for _, svc := range services {
			entries := make(chan *zeroconf.ServiceEntry)
			mdnsWg.Add(1)

			go func() {
				defer mdnsWg.Done()
				for entry := range entries {
					var ip string
					if len(entry.AddrIPv4) > 0 {
						ip = entry.AddrIPv4[0].String()
					}
					if ip == "" {
						continue
					}

					mu.Lock()
					if foundIPs[ip] {
						mu.Unlock()
						continue
					}

					// Filter for known camera manufacturers or service hints
					isCamera := false
					name := entry.Instance
					if strings.Contains(strings.ToLower(name), "camera") ||
						strings.Contains(strings.ToLower(entry.Service), "video") ||
						strings.Contains(strings.ToLower(entry.HostName), "adc") {
						isCamera = true
					}

					if isCamera {
						devices = append(devices, discovery.Device{
							ID:       ip,
							Name:     name,
							IP:       ip,
							Provider: "camera",
							Type:     "Camera",
							Model:    "IP Camera",
						})
						foundIPs[ip] = true
					}
					mu.Unlock()
				}
			}()

			_ = resolver.Browse(browseCtx, svc, "local.", entries)
		}

		<-browseCtx.Done()
		mdnsWg.Wait()
	}

	// 2. Fast Port Scan for RTSP (554) on the subnet
	// Identify local subnet
	addrs, _ := net.InterfaceAddrs()
	var targetSubnet string
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			ip := ipnet.IP.To4()
			if isPrivateIPv4(ip) {
				targetSubnet = fmt.Sprintf("%d.%d.%d.", ip[0], ip[1], ip[2])
				break
			}
		}
	}

	if targetSubnet != "" {
		var wg sync.WaitGroup
		semaphore := make(chan struct{}, 50) // Limit concurrency

		for i := 1; i < 255; i++ {
			if ctx.Err() != nil {
				break
			}
			wg.Add(1)
			go func(last int) {
				defer wg.Done()

				select {
				case <-ctx.Done():
					return
				default:
				}

				ip := fmt.Sprintf("%s%d", targetSubnet, last)

				mu.Lock()
				if foundIPs[ip] {
					mu.Unlock()
					return
				}
				mu.Unlock()

				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				d := net.Dialer{Timeout: 300 * time.Millisecond}
				conn, err := d.DialContext(ctx, "tcp", ip+":554")
				if err == nil {
					conn.Close()
					mu.Lock()
					if !foundIPs[ip] {
						devices = append(devices, discovery.Device{
							ID:       ip,
							Name:     fmt.Sprintf("Camera %s", ip),
							IP:       ip,
							Provider: "camera",
							Type:     "Camera",
							Model:    "RTSP Device",
						})
						foundIPs[ip] = true
					}
					mu.Unlock()
				}
			}(i)
		}
		wg.Wait()
	}

	return devices, nil
}
