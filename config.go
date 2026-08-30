package spectacle

import (
	"bytes"
	"fmt"
	"image"
	_ "image/png"
	"sync"
)

// Config describes the application to the backend: its identity (what the
// OS shows on windows, taskbars, toasts and .desktop entries) and where it
// keeps browser state. Pass it to New.
type Config struct {
	// ID is the machine-facing identity: a short lowercase slug ("myapp")
	// used for the window class, the Wayland app-id and .desktop entry,
	// the installed icon name, and the <ID>_THEME environment override.
	ID string
	// Name is the human-facing name ("My App"), shown on notifications
	// and used as the Windows AppUserModelID display name.
	Name string
	// Comment is a one-line description, used as the Linux .desktop
	// Comment. Optional.
	Comment string
	// Categories is the Linux .desktop Categories value, e.g.
	// "Network;InstantMessaging;". Optional.
	Categories string
	// Icon is the application icon as PNG bytes. When nil, no icon is
	// registered and notifications fall back to a generic one. Optional.
	Icon []byte
	// UserAgent overrides the webview user agent. Empty keeps the
	// backend's default (on Linux a Safari UA that passes Google OAuth's
	// browser checks; on Windows the WebView2 default). Optional.
	UserAgent string
	// DataDir hosts the browsing profiles and identity assets. Treat it
	// as per-machine cache, not roaming config.
	DataDir string
	// Debug enables DevTools in every webview.
	Debug bool
}

func (c Config) validate() error {
	if c.ID == "" || c.Name == "" || c.DataDir == "" {
		return fmt.Errorf("spectacle: Config.ID, Name and DataDir are required")
	}
	return nil
}

// conf is the running application's Config. New sets it exactly once,
// before any window or webview exists; everything after reads it.
var conf Config

// appIconImage returns the decoded Config.Icon, or nil.
var appIconImage = sync.OnceValue(func() image.Image {
	if len(conf.Icon) == 0 {
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(conf.Icon))
	if err != nil {
		return nil
	}
	return img
})
