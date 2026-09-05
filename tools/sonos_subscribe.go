//go:build ignore

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ghchinoy/homectl/pkg/sonos"
)

func main() {
	ip := flag.String("ip", "", "Sonos speaker IP")
	flag.Parse()

	if *ip == "" {
		fmt.Println("Usage: go run tools/sonos_subscribe.go -ip <IP>")
		os.Exit(1)
	}

	listener := &sonos.GENAListener{
		Handler: func(event sonos.EventMsg) {
			fmt.Printf("\n--- EVENT RECEIVED from %s ---\n", event.IP)
			if event.Volume >= 0 {
				fmt.Printf("Volume: %d%%\n", event.Volume)
			}
			if event.Status != "" {
				fmt.Printf("Status: %s\n", event.Status)
			}
			if event.Metadata.Title != "" {
				fmt.Printf("Track:  %s\n", event.Metadata.Title)
				fmt.Printf("Artist: %s\n", event.Metadata.Artist)
			}
		},
	}

	callbackURL, err := listener.Start()
	if err != nil {
		log.Fatalf("Failed to start listener: %v", err)
	}

	fmt.Printf("Listener started. Callback: %s\n", callbackURL)
	fmt.Printf("Subscribing to %s...\n", *ip)

	client := sonos.NewClient(*ip)
	sid1, err := client.Subscribe("/MediaRenderer/AVTransport/Event", callbackURL, 300)
	if err != nil {
		log.Printf("AVTransport Sub Failed: %v", err)
	} else {
		fmt.Printf("AVTransport SID: %s\n", sid1)
	}

	sid2, err := client.Subscribe("/MediaRenderer/RenderingControl/Event", callbackURL, 300)
	if err != nil {
		log.Printf("RenderingControl Sub Failed: %v", err)
	} else {
		fmt.Printf("RenderingControl SID: %s\n", sid2)
	}

	fmt.Println("Waiting for events... (Ctrl+C to quit)")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	fmt.Println("\nShutting down...")
}
