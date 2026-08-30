//go:build linux

package spectacle

import (
	"github.com/pietjan/spectacle/internal/native"
)

// window is the GTK4 top-level window with a GtkFixed content host.
type window struct {
	backend *backend
	win     uintptr // GtkWindow*
	fixed   uintptr // GtkFixed*

	onResize func(w, h, dpi int)
	onClose  func() bool

	lastW, lastH, lastScale int
	resizePending           bool

	tray *tray
}

func newWindow(b *backend, title string, bounds Rect) (*window, error) {
	w := &window{backend: b}
	w.win = native.GObjectRefSink(native.GtkWindowNew())
	native.GtkWindowSetTitle(w.win, title)
	native.GtkWindowSetIconName(w.win, conf.ID)
	// GTK4 removed window positioning outright; only size applies.
	// Sizes are logical px; scale is 1 pre-realize and the first layout
	// pass corrects any difference.
	if bounds.W > 0 && bounds.H > 0 {
		native.GtkWindowSetDefaultSize(w.win, int32(bounds.W), int32(bounds.H))
	}
	// A client-side header bar, so the theme CSS owns the titlebar too.
	native.GtkWindowSetTitlebar(w.win, native.GtkHeaderBarNew())
	w.fixed = native.GtkFixedNew()
	native.GtkWindowSetChild(w.win, w.fixed)

	native.Connect(w.win, "close-request", 0, func([]uintptr) uintptr {
		if w.onClose != nil && !w.onClose() {
			return 1 // veto; the handler hid the window
		}
		return 0
	})
	native.Connect(w.win, "destroy", 0, func([]uintptr) uintptr {
		b.Quit()
		return 0
	})
	// Size tracking: GTK4 has no size-allocate signal. GdkSurface::layout
	// fires on every geometry change once realized; the notifies cover
	// pre-realize and maximize edges. Each schedules one coalesced check
	// at default-idle priority, which sorts after GTK's layout phase, so
	// widget sizes read fresh.
	native.Connect(w.win, "realize", 0, func([]uintptr) uintptr {
		if surface := native.GtkNativeGetSurface(native.GtkWidgetGetNative(w.win)); surface != 0 {
			native.Connect(surface, "layout", 2, func([]uintptr) uintptr {
				w.scheduleResizeCheck()
				return 0
			})
		}
		w.scheduleResizeCheck()
		return 0
	})
	for _, sig := range []string{"notify::maximized", "notify::default-width", "notify::default-height"} {
		native.Connect(w.win, sig, 1, func([]uintptr) uintptr {
			w.scheduleResizeCheck()
			return 0
		})
	}
	return w, nil
}

func (w *window) scheduleResizeCheck() {
	if w.resizePending {
		return
	}
	w.resizePending = true
	native.ScheduleIdle(native.PriorityDefaultIdle, func() {
		w.resizePending = false
		w.checkResize()
	})
}

func (w *window) checkResize() {
	if w.onResize == nil {
		return
	}
	scale := int(native.GtkWidgetGetScaleFactor(w.win))
	if scale < 1 {
		scale = 1
	}
	cw := int(native.GtkWidgetGetWidth(w.fixed)) * scale
	ch := int(native.GtkWidgetGetHeight(w.fixed)) * scale
	if cw == 0 || ch == 0 {
		// Not yet allocated: report the requested size so the app can
		// lay out before the first frame.
		var dw, dh int32
		native.GtkWindowGetDefaultSize(w.win, &dw, &dh)
		cw, ch = int(dw)*scale, int(dh)*scale
	}
	if cw == w.lastW && ch == w.lastH && scale == w.lastScale {
		return
	}
	w.lastW, w.lastH, w.lastScale = cw, ch, scale
	w.onResize(cw, ch, 96*scale)
}

// SetBounds resizes the window (position is not a thing on GTK4).
func (w *window) SetBounds(r Rect) {
	scale := max(int(native.GtkWidgetGetScaleFactor(w.win)), 1)
	native.GtkWindowSetDefaultSize(w.win, int32(r.W/scale), int32(r.H/scale))
}

// Bounds returns {0,0,size}: window positions are unknowable on GTK4.
// The size round-trips through the app's geometry persistence.
func (w *window) Bounds() Rect {
	scale := max(int(native.GtkWidgetGetScaleFactor(w.win)), 1)
	var dw, dh int32
	native.GtkWindowGetDefaultSize(w.win, &dw, &dh)
	return Rect{W: int(dw) * scale, H: int(dh) * scale}
}

// Show presents the window.
func (w *window) Show() { native.GtkWindowPresent(w.win) }

// Hide hides the window (it keeps running; the tray brings it back).
func (w *window) Hide() { native.GtkWidgetSetVisible(w.win, 0) }

// IsVisible reports whether the window is shown.
func (w *window) IsVisible() bool { return native.GtkWidgetGetVisible(w.win) != 0 }

// OnResize registers the resize callback and fires it once so layout
// starts correct.
func (w *window) OnResize(fn func(width, height, dpi int)) {
	w.onResize = fn
	w.checkResize()
}

// OnCloseRequest registers the close-veto callback.
func (w *window) OnCloseRequest(fn func() bool) { w.onClose = fn }

// Tray returns (creating on first use) the status notifier item.
func (w *window) Tray() Tray {
	if w.tray == nil {
		w.tray = newTray(w)
	}
	return w.tray
}
