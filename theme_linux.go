//go:build linux

package spectacle

import (
	"os"
	"strings"
	"sync"

	"github.com/pietjan/spectacle/internal/native"
)

// Dark reports whether the app should present dark: <ID>_THEME env
// override, then the GNOME color-scheme setting, defaulting to dark
// (the app's design leans dark, and most desktops without the schema
// give no signal either way).
var Dark = sync.OnceValue(func() bool {
	switch os.Getenv(strings.ToUpper(conf.ID) + "_THEME") {
	case "dark":
		return true
	case "light":
		return false
	}
	// Only an explicit preference counts; "default" means the desktop
	// has no opinion and the design leans dark.
	switch scheme := native.SettingsString("org.gnome.desktop.interface", "color-scheme"); {
	case strings.Contains(scheme, "dark"):
		return true
	case strings.Contains(scheme, "light"):
		return false
	}
	return true
})

// applyTheme makes GTK follow the choice (so pages see the matching
// prefers-color-scheme) and paints exposed window area zinc-900 — the
// Linux analogue of the Win32 class brush.
func applyTheme() {
	if !Dark() {
		return
	}
	if settings := native.GtkSettingsGetDefault(); settings != 0 {
		native.SetBoolProperty(settings, "gtk-application-prefer-dark-theme", true)
	}
	provider := native.GtkCssProviderNew()
	native.GtkCssProviderLoadFromString(provider, `
		window { background-color: #18181b; }
		headerbar {
			background: #18181b;
			color: #f4f4f5;
			border: none;
			box-shadow: none;
			min-height: 38px;
		}
		headerbar windowtitle { color: #f4f4f5; }
	`)
	if display := native.GdkDisplayGetDefault(); display != 0 {
		native.GtkStyleContextAddProviderForDisplay(display, provider, native.StyleProviderPriorityApplication)
	}
}
