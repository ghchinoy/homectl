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

type dialContextFunc func(ctx context.Context, network, address string) (net.Conn, error)

var defaultDialContext dialContextFunc = func(ctx context.Context, network, address string) (net.Conn, error) {
	d := net.Dialer{Timeout: 300 * time.Millisecond}
	return d.DialContext(ctx, network, address)
}

func findPrivateSubnet() string {
	addrs, _ := net.InterfaceAddrs()
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			ip := ipnet.IP.To4()
			if isPrivateIPv4(ip) {
				return fmt.Sprintf("%d.%d.%d.", ip[0], ip[1], ip[2])
			}
		}
	}
	return ""
}

func (p *DiscoveryProvider) Discover(ctx context.Context) ([]discovery.Device, error) {
	var devices []discovery.Device
	foundIPs := make(map[string]bool)
	var mu sync.Mutex

	timeout := 2 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}

	// Run mDNS and RTSP port 554 scan concurrently to ensure port probe
	// is not starved by mDNS timeout.
	var outerWg sync.WaitGroup

	// 1. mDNS Discovery (runs concurrently)
	outerWg.Add(1)
	go func() {
		defer outerWg.Done()
		p.discoverMDNS(ctx, timeout, foundIPs, &devices, &mu)
	}()

	// 2. Fast Port Scan for RTSP (554) on subnet (runs concurrently)
	outerWg.Add(1)
	go func() {
		defer outerWg.Done()
		p.scanSubnetRTSP(ctx, findPrivateSubnet(), defaultDialContext, foundIPs, &devices, &mu)
	}()

	outerWg.Wait()
	return devices, nil
}

func (p *DiscoveryProvider) discoverMDNS(ctx context.Context, timeout time.Duration, foundIPs map[string]bool, devices *[]discovery.Device, mu *sync.Mutex) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return
	}

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
					*devices = append(*devices, discovery.Device{
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

func (p *DiscoveryProvider) scanSubnetRTSP(ctx context.Context, targetSubnet string, dialer dialContextFunc, foundIPs map[string]bool, devices *[]discovery.Device, mu *sync.Mutex) {
	if targetSubnet == "" {
		return
	}

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

			conn, err := dialer(ctx, "tcp", ip+":554")
			if err == nil {
				conn.Close()
				mu.Lock()
				if !foundIPs[ip] {
					*devices = append(*devices, discovery.Device{
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
