package main

import (
	"fmt"
	"net"
	"time"
)

func main() {
	ssdpAddr, _ := net.ResolveUDPAddr("udp4", "239.255.255.250:1900")
	conn, _ := net.ListenPacket("udp4", ":0")
	defer conn.Close()

	msg := "M-SEARCH * HTTP/1.1\r\n" +
		"HOST: 239.255.255.250:1900\r\n" +
		"MAN: \"ssdp:discover\"\r\n" +
		"MX: 1\r\n" +
		"ST: ssdp:all\r\n" +
		"\r\n"

	fmt.Println("Sending discovery message...")
	conn.WriteTo([]byte(msg), ssdpAddr)

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for {
		buf := make([]byte, 2048)
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			fmt.Printf("Done or Error: %v\n", err)
			break
		}
		fmt.Printf("Response from %s:\n%s\n---\n", addr, string(buf[:n]))
	}
}

