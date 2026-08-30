//go:build linux

package spectacle

import (
	"log"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
)

const (
	sniPath      = "/StatusNotifierItem"
	sniInterface = "org.kde.StatusNotifierItem"
	watcherName  = "org.kde.StatusNotifierWatcher"
)

// sniItem exports org.kde.StatusNotifierItem — the freedesktop tray
// protocol (KDE and most shells; GNOME needs the AppIndicator
// extension).
type sniItem struct {
	backend    *backend
	conn       *dbus.Conn
	props      *prop.Properties
	onActivate func()
	registered bool
}

// tooltip is the SNI ToolTip type (s a(iiay) s s).
type tooltip struct {
	IconName string
	Pixmaps  []sniPixmap
	Title    string
	Text     string
}

func newSNI(b *backend, conn *dbus.Conn) *sniItem {
	s := &sniItem{backend: b, conn: conn}
	if err := conn.Export(s, sniPath, sniInterface); err != nil {
		log.Printf("linux: sni export: %v", err)
		return s
	}
	var pixmaps []sniPixmap
	if img := appIconImage(); img != nil {
		pixmaps = sniPixmaps(img, 16, 22, 24, 32, 48)
	}
	props, err := prop.Export(conn, sniPath, prop.Map{
		sniInterface: {
			"Category":   {Value: "ApplicationStatus", Emit: prop.EmitTrue},
			"Id":         {Value: conf.ID, Emit: prop.EmitTrue},
			"Title":      {Value: conf.Name, Emit: prop.EmitTrue},
			"Status":     {Value: "Active", Emit: prop.EmitTrue},
			"IconName":   {Value: "", Emit: prop.EmitTrue},
			"IconPixmap": {Value: pixmaps, Emit: prop.EmitTrue},
			"ToolTip":    {Value: tooltip{Title: conf.Name}, Emit: prop.EmitTrue},
			"Menu":       {Value: dbus.ObjectPath(menuPath), Emit: prop.EmitTrue},
			"ItemIsMenu": {Value: false, Emit: prop.EmitTrue},
		},
	})
	if err != nil {
		log.Printf("linux: sni props: %v", err)
		return s
	}
	s.props = props
	s.register()
	s.watchWatcher()
	return s
}

// register announces the item to the StatusNotifierWatcher, if any.
func (s *sniItem) register() {
	watcher := s.conn.Object(watcherName, "/StatusNotifierWatcher")
	call := watcher.Call(watcherName+".RegisterStatusNotifierItem", 0, s.conn.Names()[0])
	s.registered = call.Err == nil
	if call.Err != nil {
		// No watcher (GNOME without extension, WSLg): the item stays
		// exported; a watcher appearing later picks us up via the
		// NameOwnerChanged re-register below.
		log.Printf("linux: no status notifier watcher: %v", call.Err)
	}
}

// watchWatcher re-registers when the watcher (re)appears — shell
// restarts would otherwise lose the tray icon.
func (s *sniItem) watchWatcher() {
	err := s.conn.AddMatchSignal(
		dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
		dbus.WithMatchArg(0, watcherName),
	)
	if err != nil {
		return
	}
	ch := make(chan *dbus.Signal, 4)
	s.conn.Signal(ch)
	go func() {
		for sig := range ch {
			if sig.Name != "org.freedesktop.DBus.NameOwnerChanged" || len(sig.Body) < 3 {
				continue
			}
			if newOwner, _ := sig.Body[2].(string); newOwner != "" {
				s.register()
			}
		}
	}()
}

func (s *sniItem) setTooltip(text string) {
	if s.props != nil {
		s.props.SetMust(sniInterface, "ToolTip", tooltip{Title: conf.Name, Text: text})
		s.conn.Emit(sniPath, sniInterface+".NewToolTip")
	}
}

// Activate implements org.kde.StatusNotifierItem.Activate (icon click).
func (s *sniItem) Activate(x, y int32) *dbus.Error {
	if s.onActivate != nil {
		s.backend.Dispatch(s.onActivate)
	}
	return nil
}

// SecondaryActivate implements the middle-click method.
func (s *sniItem) SecondaryActivate(x, y int32) *dbus.Error { return nil }

// ContextMenu is host-rendered from the Menu property; nothing to do.
func (s *sniItem) ContextMenu(x, y int32) *dbus.Error { return nil }

// Scroll implements the scroll method.
func (s *sniItem) Scroll(delta int32, orientation string) *dbus.Error { return nil }
