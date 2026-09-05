//go:build ignore

package main

import (
	"fmt"
	"log"
	"os"

	"github.com/ghchinoy/homectl/modules/sonos"
)

func main() {
	targetIP := ""
	if len(os.Args) > 1 {
		targetIP = os.Args[1]
	} else if env := os.Getenv("HOMECTL_SONOS_IP"); env != "" {
		targetIP = env
	} else {
		cached, _ := sonos.LoadCache()
		if len(cached) > 0 {
			targetIP = cached[0].IP
		}
	}
	if targetIP == "" {
		log.Fatal("Usage: test_sonos <speaker_ip> or set HOMECTL_SONOS_IP")
	}

	client := sonos.NewClient(targetIP)

	fmt.Printf("Querying volume for speaker at %s...\n", targetIP)
	vol, err := client.GetVolume()
	if err != nil {
		log.Fatalf("Failed to get volume: %v", err)
	}

	fmt.Printf("Current Volume: %d%%\n", vol)
}
