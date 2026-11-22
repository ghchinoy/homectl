module github.com/ghchinoy/homectl/modules/roborock

go 1.26.11

require (
	github.com/ghchinoy/homectl/modules/core v0.0.0
	github.com/mochi-mqtt/server/v2 v2.7.9
	github.com/pion/turn/v4 v4.1.4
)

require (
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/pion/dtls/v3 v3.0.7 // indirect
	github.com/pion/logging v0.2.4 // indirect
	github.com/pion/randutil v0.1.0 // indirect
	github.com/pion/stun/v3 v3.0.1 // indirect
	github.com/pion/transport/v3 v3.0.8 // indirect
	github.com/pion/transport/v4 v4.0.1 // indirect
	github.com/rs/xid v1.4.0 // indirect
	github.com/wlynxg/anet v0.0.5 // indirect
	golang.org/x/crypto v0.32.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
)

replace github.com/ghchinoy/homectl/modules/core => ../core
