//go:build ignore

package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/ghchinoy/homectl/modules/sonos"
)

func main() {
	ip := ""
	if len(os.Args) > 1 {
		ip = os.Args[1]
	} else if env := os.Getenv("HOMECTL_SONOS_IP"); env != "" {
		ip = env
	} else {
		cached, _ := sonos.LoadCache()
		if len(cached) > 0 {
			ip = cached[0].IP
		}
	}
	if ip == "" {
		log.Fatal("Usage: dump_sonos_metadata <speaker_ip> or set HOMECTL_SONOS_IP")
	}
	client := sonos.NewClient(ip)

	fmt.Printf("--- Raw Position Info for %s ---\n", ip)
	body, err := client.SOAPAction(
		"/MediaRenderer/AVTransport/Control",
		"urn:schemas-upnp-org:service:AVTransport:1",
		"GetPositionInfo",
		map[string]interface{}{
			"InstanceID": 0,
		})
	if err != nil {
		log.Fatalf("Failed to get position info: %v", err)
	}

	d1, _ := io.ReadAll(body)
	body.Close()
	fmt.Println(string(d1))

	fmt.Printf("\n--- Raw Media Info for %s ---\n", ip)
	body2, err := client.SOAPAction(
		"/MediaRenderer/AVTransport/Control",
		"urn:schemas-upnp-org:service:AVTransport:1",
		"GetMediaInfo",
		map[string]interface{}{
			"InstanceID": 0,
		})
	if err != nil {
		log.Fatalf("Failed to get media info: %v", err)
	}

	d2, _ := io.ReadAll(body2)
	body2.Close()
	fmt.Println(string(d2))
}
