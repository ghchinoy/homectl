module github.com/ghchinoy/homectl

go 1.26.10

require (
	github.com/charmbracelet/bubbles v1.0.0
	github.com/charmbracelet/bubbletea v1.3.10
	github.com/charmbracelet/lipgloss v1.1.0
	github.com/grandcat/zeroconf v1.0.0
	github.com/spf13/cobra v1.10.2
)

require (
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
)

require (
	github.com/google/jsonschema-go v0.4.3 // indirect
	github.com/modelcontextprotocol/go-sdk v1.8.0
	github.com/segmentio/asm v1.2.1 // indirect
	github.com/segmentio/encoding v0.5.4 // indirect
	github.com/yosida95/uritemplate/v3 v3.0.2 // indirect
	golang.org/x/oauth2 v0.37.0 // indirect
	golang.org/x/time v0.16.0 // indirect
	golang.org/x/tools v0.52.0 // indirect
)

require (
	github.com/atotto/clipboard v0.1.4 // indirect
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/buger/jsonparser v1.6.1 // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/charmbracelet/colorprofile v0.4.3 // indirect
	github.com/charmbracelet/harmonica v0.2.0 // indirect
	github.com/charmbracelet/x/ansi v0.11.9 // indirect
	github.com/charmbracelet/x/cellbuf v0.0.15 // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/erikgeiser/coninput v0.0.0-20211004153227-1c3628e74d0f // indirect
	github.com/ghchinoy/homectl/modules/core v0.0.0
	github.com/ghchinoy/homectl/modules/sonos v0.0.0
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/h2non/filetype v1.1.3 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.1 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/mattn/go-localereader v0.0.1 // indirect
	github.com/mattn/go-runewidth v0.0.31 // indirect
	github.com/miekg/dns v1.1.73 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/muesli/ansi v0.0.0-20230316100256-276c6243b2f6 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/muesli/termenv v0.16.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/sahilm/fuzzy v0.1.3 // indirect
	github.com/sirupsen/logrus v1.10.2 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/vishen/go-chromecast v0.3.4
	github.com/xo/terminfo v1.3.0 // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	gopkg.in/ini.v1 v1.67.3 // indirect
	nhooyr.io/websocket v1.8.17
)

replace github.com/ghchinoy/homectl/modules/core => ./modules/core

replace github.com/ghchinoy/homectl/modules/sonos => ./modules/sonos
