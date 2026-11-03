module github.com/ghchinoy/homectl/modules/sonos

go 1.26.10

require (
	github.com/ghchinoy/homectl/modules/core v0.0.0
	github.com/grandcat/zeroconf v1.0.0
)

require (
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/miekg/dns v1.1.73 // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
)

replace github.com/ghchinoy/homectl/modules/core => ../core
