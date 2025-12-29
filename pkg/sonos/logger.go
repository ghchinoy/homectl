package sonos

import (
	"log"
	"os"

	"github.com/ghchinoy/homectl/pkg/config"
)

var sonosLogger *log.Logger

func init() {
	config.EnsureDir()
	f, _ := os.OpenFile(config.GetPath("sonos.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if f != nil {
		sonosLogger = log.New(f, "SONOS: ", log.LstdFlags)
	}
}
