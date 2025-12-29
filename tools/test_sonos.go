package main

import (
	"fmt"
	"log"

	"github.com/ghchinoy/homectl/pkg/sonos"
)

func main() {
	move2IP := "192.168.4.120"
	client := sonos.NewClient(move2IP)

	fmt.Printf("Querying volume for Move 2 at %s...\n", move2IP)
	vol, err := client.GetVolume()
	if err != nil {
		log.Fatalf("Failed to get volume: %v", err)
	}

	fmt.Printf("Current Volume: %d%%\n", vol)
}
