//go:build ignore

package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"sort"
	"syscall"
	"time"

	"github.com/ghchinoy/homectl/modules/sonos"
)

func main() {
	targetIP := flag.String("ip", "", "Sonos speaker IP (e.g. 192.168.1.120)")
	callbackIP := flag.String("callback-ip", "", "Manual callback IP (if NATed/Container)")
	port := flag.Int("port", 0, "Manual port (0 for random)")
	flag.Parse()

	if *targetIP == "" {
		fmt.Println("Usage: go run tools/gena_debug.go -ip <Sonos_IP> [-callback-ip <My_LAN_IP>] [-port <Port>]")
		printLocalIPs()
		os.Exit(1)
	}

	// 1. Setup Listener
	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	actualPort := l.Addr().(*net.TCPAddr).Port

	// 2. Determine Callback URL
	cbIP := *callbackIP
	if cbIP == "" {
		cbIP = guessLocalIP()
	}
	callbackURL := fmt.Sprintf("http://%s:%d/", cbIP, actualPort)

	fmt.Printf("---" + "DIAGNOSTIC START" + "---\n")
	fmt.Printf("Listening on: %s\n", l.Addr())
	fmt.Printf("Callback URL: %s\n", callbackURL)
	fmt.Printf("Target Speaker: %s\n", *targetIP)
	fmt.Printf("------------------------\n\n")

	// 3. Start HTTP server with RAW logging
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			dump, _ := httputil.DumpRequest(r, true)
			fmt.Printf("\n[%s] INCOMING REQUEST:\n%s\n", time.Now().Format("15:04:05"), string(dump))
			w.WriteHeader(http.StatusOK)
		})
		if err := http.Serve(l, mux); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()

	// 4. Subscribe
	client := sonos.NewClient(*targetIP)
	fmt.Printf("Subscribing to AVTransport events...\n")
	sid, err := client.Subscribe("/MediaRenderer/AVTransport/Event", callbackURL, 60)
	if err != nil {
		fmt.Printf("!! SUBSCRIPTION FAILED: %v\n", err)
		fmt.Println("This usually means the Sonos speaker cannot reach your Callback URL.")
	} else {
		fmt.Printf("✓ SUBSCRIPTION SUCCESSFUL! SID: %s\n", sid)
		fmt.Println("Now try playing/pausing music in the Sonos app. Events should appear below.")
	}

	// Wait for signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	fmt.Println("\nShutting down...")
}

func guessLocalIP() string {
	addrs, _ := net.InterfaceAddrs()
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return "127.0.0.1"
}

func printLocalIPs() {
	fmt.Println("\nAvailable Local IPs:")
	addrs, _ := net.InterfaceAddrs()
	var list []string
	for _, addr := range addrs {
		list = append(list, addr.String())
	}
	sort.Strings(list)
	for _, a := range list {
		fmt.Printf(" - %s\n", a)
	}
}
