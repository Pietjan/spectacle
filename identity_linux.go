//go:build linux

package spectacle

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// registerIdentity installs the app's desktop identity for the current
// user — the Linux analogue of the Windows AUMID registration. Taskbars
// and shells resolve a window's app-id (Config.ID, via g_set_prgname)
// against a .desktop entry and the icon theme; neither exists for an
// app that was never packaged, so install both. Best-effort.
func registerIdentity() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	if len(conf.Icon) > 0 {
		iconDir := filepath.Join(home, ".local/share/icons/hicolor/256x256/apps")
		if err := os.MkdirAll(iconDir, 0o755); err == nil {
			if err := os.WriteFile(filepath.Join(iconDir, conf.ID+".png"), conf.Icon, 0o644); err != nil {
				log.Printf("linux: app icon: %v", err)
			}
		}
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	appsDir := filepath.Join(home, ".local/share/applications")
	if err := os.MkdirAll(appsDir, 0o755); err != nil {
		return
	}
	desktop := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=%s
Comment=%s
Exec=%s
Icon=%s
Categories=%s
StartupWMClass=%s
`, conf.Name, conf.Comment, exe, conf.ID, conf.Categories, conf.ID)
	if err := os.WriteFile(filepath.Join(appsDir, conf.ID+".desktop"), []byte(desktop), 0o644); err != nil {
		log.Printf("linux: desktop entry: %v", err)
	}
}
