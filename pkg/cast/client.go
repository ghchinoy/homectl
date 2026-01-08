package cast

import (
	"time"

	"github.com/vishen/go-chromecast/application"
)

// Status represents the current state of a Google Cast device
type Status struct {
	AppId       string  `json:"app_id"`
	DisplayName string  `json:"display_name"`
	Volume      float64 `json:"volume"`
	IsMuted     bool    `json:"is_muted"`
	StatusText  string  `json:"status_text"`
}

// GetStatus connects to a Cast device and retrieves its current status
func GetStatus(ip string) (Status, error) {
	// Use NewApplication to ensure internal fields (like Storage) are initialized
	app := application.NewApplication(application.WithCacheDisabled(true))
	if err := app.Start(ip, 8009); err != nil {
		return Status{}, err
	}
	defer app.Close(false)

	// Wait for status update
	time.Sleep(500 * time.Millisecond)

	appStatus, _, volStatus := app.Status()

	res := Status{}

	if appStatus != nil {
		res.AppId = appStatus.AppId
		res.DisplayName = appStatus.DisplayName
		res.StatusText = appStatus.StatusText
	}

	if volStatus != nil {
		res.Volume = float64(volStatus.Level * 100)
		res.IsMuted = volStatus.Muted
	}

	return res, nil
}

// Control sends a command to a Cast device
func Control(ip, action string, volume float64) error {
	app := application.NewApplication(application.WithCacheDisabled(true))
	if err := app.Start(ip, 8009); err != nil {
		return err
	}
	defer app.Close(false)

	switch action {
	case "play":
		return app.Unpause()
	case "pause":
		return app.Pause()
	case "stop":
		return app.Stop()
	case "volume":
		// Volume is 0-100, SDK expects 0.0-1.0
		return app.SetVolume(float32(volume / 100.0))
	case "mute":
		return app.SetMuted(true)
	case "unmute":
		return app.SetMuted(false)
	}

	return nil
}