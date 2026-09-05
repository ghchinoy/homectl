//go:build ignore

package main

import (
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run tools/debug_miio.go <IP>")
		return
	}
	ip := os.Args[1]
	target := ip + ":54321"
	handshake, _ := hex.DecodeString("21310020ffffffffffffffffffffffffffffffffffffffffffffffffffffffff")

	conn, err := net.Dial("udp", target)
	if err != nil {
		fmt.Printf("Dial error: %v\n", err)
		return
	}
	defer conn.Close()

	fmt.Printf("Sending handshake to %s...\n", target)
	conn.Write(handshake)

	buf := make([]byte, 64)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buf)
	if err != nil {
		fmt.Printf("Read error: %v\n", err)
		return
	}

	fmt.Printf("Received %d bytes: %x\n", n, buf[:n])
	if n >= 32 {
		deviceID := hex.EncodeToString(buf[8:12])
		fmt.Printf("Device ID: %s\n", deviceID)
	}
}
