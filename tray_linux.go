//go:build linux

package spectacle

import (
	"image"
	"log"

	"github.com/godbus/dbus/v5"
)

// tray is the Linux tray: a StatusNotifierItem + dbusmenu for the icon
// and menu, org.freedesktop.Notifications for toasts. Without a session
// bus every method degrades to a logged no-op; without a
// StatusNotifierWatcher (GNOME sans extension) the icon is invisible
// but notifications still work — close-to-tray then relies on them as
// the re-entry point.
type tray struct {
	backend  *backend
	conn     *dbus.Conn
	sni      *sniItem
	menu     *dbusMenu
	notifier *notifier

	onActivate    func()
	onNotifyClick func()
}

func newTray(w *window) *tray {
	t := &tray{backend: w.backend}
	conn, err := dbus.SessionBus()
	if err != nil {
		log.Printf("linux: no session bus, tray and notifications disabled: %v", err)
		return t
	}
	t.conn = conn
	t.menu = newDBusMenu(t.backend, conn)
	t.sni = newSNI(t.backend, conn)
	t.sni.onActivate = func() {
		if t.onActivate != nil {
			t.onActivate()
		}
	}
	t.notifier = newNotifier(t.backend, conn)
	t.notifier.onClick = func() {
		switch {
		case t.onNotifyClick != nil:
			t.onNotifyClick()
		case t.onActivate != nil:
			t.onActivate()
		}
	}
	return t
}

// Available reports whether a StatusNotifierWatcher accepted the icon —
// without one (GNOME sans extension, WSLg) there is no tray to hide to.
func (t *tray) Available() bool { return t.sni != nil && t.sni.registered }

// SetTooltip sets the hover text.
func (t *tray) SetTooltip(tip string) {
	if t.sni != nil {
		t.sni.setTooltip(tip)
	}
}

// SetMenu sets the tray context menu.
func (t *tray) SetMenu(items []MenuItem) {
	if t.menu != nil {
		t.menu.setItems(items)
	}
}

// Notify shows a desktop notification.
func (t *tray) Notify(title, body string, icon image.Image) {
	if t.notifier != nil {
		t.notifier.notify(title, body, icon)
	}
}

// OnActivate registers the icon-click handler.
func (t *tray) OnActivate(fn func()) { t.onActivate = fn }

// OnNotificationClick registers the notification-click handler.
func (t *tray) OnNotificationClick(fn func()) { t.onNotifyClick = fn }

// remove releases the bus connection on shutdown.
func (t *tray) remove() {
	if t.conn != nil {
		t.conn.Close()
	}
}
