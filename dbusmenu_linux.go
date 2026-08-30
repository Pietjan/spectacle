//go:build linux

package spectacle

import (
	"log"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
)

const menuPath = "/MenuBar"

// dbusMenu exports the tray's context menu via com.canonical.dbusmenu —
// the host shell renders it from GetLayout; clicks come back as Events.
type dbusMenu struct {
	backend  *backend
	conn     *dbus.Conn
	items    []MenuItem
	revision uint32
}

// layoutNode is the dbusmenu (ia{sv}av) structure.
type layoutNode struct {
	ID       int32
	Props    map[string]dbus.Variant
	Children []dbus.Variant
}

func newDBusMenu(b *backend, conn *dbus.Conn) *dbusMenu {
	m := &dbusMenu{backend: b, conn: conn}
	if err := conn.Export(m, menuPath, "com.canonical.dbusmenu"); err != nil {
		log.Printf("linux: dbusmenu export: %v", err)
		return m
	}
	_, err := prop.Export(conn, menuPath, prop.Map{
		"com.canonical.dbusmenu": {
			"Version":       {Value: uint32(3), Emit: prop.EmitTrue},
			"Status":        {Value: "normal", Emit: prop.EmitTrue},
			"TextDirection": {Value: "ltr", Emit: prop.EmitTrue},
			"IconThemePath": {Value: "", Emit: prop.EmitTrue},
		},
	})
	if err != nil {
		log.Printf("linux: dbusmenu props: %v", err)
	}
	return m
}

func (m *dbusMenu) setItems(items []MenuItem) {
	m.items = items
	m.revision++
	m.conn.Emit(menuPath, "com.canonical.dbusmenu.LayoutUpdated", m.revision, int32(0))
}

func (m *dbusMenu) node(i int) layoutNode {
	item := m.items[i]
	props := map[string]dbus.Variant{}
	if item.Label == "" {
		props["type"] = dbus.MakeVariant("separator")
	} else {
		props["label"] = dbus.MakeVariant(item.Label)
	}
	return layoutNode{ID: int32(i + 1), Props: props}
}

// GetLayout implements com.canonical.dbusmenu.GetLayout.
func (m *dbusMenu) GetLayout(parentID, depth int32, propertyNames []string) (uint32, layoutNode, *dbus.Error) {
	root := layoutNode{
		ID:    0,
		Props: map[string]dbus.Variant{"children-display": dbus.MakeVariant("submenu")},
	}
	if parentID == 0 {
		for i := range m.items {
			root.Children = append(root.Children, dbus.MakeVariant(m.node(i)))
		}
	}
	return m.revision, root, nil
}

// GetGroupProperties implements com.canonical.dbusmenu.GetGroupProperties.
func (m *dbusMenu) GetGroupProperties(ids []int32, propertyNames []string) ([]struct {
	ID    int32
	Props map[string]dbus.Variant
}, *dbus.Error) {
	var out []struct {
		ID    int32
		Props map[string]dbus.Variant
	}
	for _, id := range ids {
		i := int(id) - 1
		if i < 0 || i >= len(m.items) {
			continue
		}
		n := m.node(i)
		out = append(out, struct {
			ID    int32
			Props map[string]dbus.Variant
		}{n.ID, n.Props})
	}
	return out, nil
}

// GetProperty implements com.canonical.dbusmenu.GetProperty.
func (m *dbusMenu) GetProperty(id int32, name string) (dbus.Variant, *dbus.Error) {
	i := int(id) - 1
	if i >= 0 && i < len(m.items) {
		if v, ok := m.node(i).Props[name]; ok {
			return v, nil
		}
	}
	return dbus.MakeVariant(""), nil
}

// Event implements com.canonical.dbusmenu.Event (clicks).
func (m *dbusMenu) Event(id int32, eventID string, data dbus.Variant, timestamp uint32) *dbus.Error {
	if eventID != "clicked" {
		return nil
	}
	i := int(id) - 1
	if i >= 0 && i < len(m.items) && m.items[i].OnClick != nil {
		// DBus goroutine → UI thread.
		m.backend.Dispatch(m.items[i].OnClick)
	}
	return nil
}

// EventGroup implements com.canonical.dbusmenu.EventGroup.
func (m *dbusMenu) EventGroup(events []struct {
	ID        int32
	EventID   string
	Data      dbus.Variant
	Timestamp uint32
}) ([]int32, *dbus.Error) {
	for _, e := range events {
		m.Event(e.ID, e.EventID, e.Data, e.Timestamp)
	}
	return nil, nil
}

// AboutToShow implements com.canonical.dbusmenu.AboutToShow.
func (m *dbusMenu) AboutToShow(id int32) (bool, *dbus.Error) { return false, nil }

// AboutToShowGroup implements com.canonical.dbusmenu.AboutToShowGroup.
func (m *dbusMenu) AboutToShowGroup(ids []int32) ([]int32, []int32, *dbus.Error) {
	return nil, nil, nil
}
