//go:build windows

package spectacle

import (
	"log"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows/registry"

	"github.com/pietjan/spectacle/internal/w32"
)

// registerIdentity sets the process AUMID (Config.Name) and registers its
// display name and icon, which is what puts a name and icon on toasts from
// an app that was never installed. dataDir hosts the icon file. Best-effort.
func registerIdentity(dataDir string) {
	w32.SetAppUserModelID.Call(uintptr(unsafe.Pointer(utf16Ptr(conf.Name))))

	k, _, err := registry.CreateKey(registry.CURRENT_USER,
		`Software\Classes\AppUserModelId\`+conf.Name, registry.SET_VALUE)
	if err != nil {
		log.Printf("win: app identity: %v", err)
		return
	}
	defer k.Close()
	k.SetStringValue("DisplayName", conf.Name)
	if len(conf.Icon) == 0 {
		return
	}
	iconPath := filepath.Join(dataDir, "appicon.png")
	if err := os.WriteFile(iconPath, conf.Icon, 0o644); err != nil {
		log.Printf("win: app icon: %v", err)
		return
	}
	k.SetStringValue("IconUri", iconPath)
}
