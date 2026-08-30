//go:build linux

package spectacle

import (
	"github.com/pietjan/spectacle/internal/native"
)

// window is the GTK4 top-level window with a GtkFixed content host.
type window struct {
	backend *backend
	win     uintptr // GtkWindow*
	clip    uintptr // GtkScrolledWindow* wrapping fixed
	fixed   uintptr // GtkFixed*

	onResize func(w, h, dpi int)
	onClose  func() bool

	lastW, lastH, lastScale int
	resizePending           bool

	// Input-region support: overlays stay stacked above normal views,
	// and views with input regions get can-target toggled from pointer
	// tracking (see pointerMoved).
	overlays    []*webView
	regionViews []*webView
	ptrX, ptrY  float64 // last pointer position, client-area logical px
	ptrKnown    bool

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
	// Webview sizes are driven by size requests, which double as
	// minimums — hosting the fixed directly would stop the window from
	// ever shrinking (the minimum ratchets up to the current size). A
	// scrolled window with external policy absorbs the child minimum
	// (no scrollbars, nothing actually scrolls); the app re-lays views
	// out to the new size on every resize anyway.
	w.fixed = native.GtkFixedNew()
	w.clip = native.GtkScrolledWindowNew()
	native.GtkScrolledWindowSetPolicy(w.clip, native.PolicyExternal, native.PolicyExternal)
	native.GtkScrolledWindowSetChild(w.clip, w.fixed)
	native.GtkWindowSetChild(w.win, w.clip)

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
	// Pointer tracking for SetInputRegions. GTK4 picks the event target
	// per widget, all-or-nothing (can-target), so region-limited views
	// get their can-target flipped from the pointer position instead.
	// A capture-phase controller on the window sees every pointer event
	// before picking routes the NEXT one — and a press is always
	// preceded by the motion that brought the pointer there, so the flag
	// is correct by the time clicks arrive.
	ptr := native.GtkEventControllerLegacyNew()
	native.GtkEventControllerSetPropagationPhase(ptr, native.PhaseCapture)
	native.Connect(ptr, "event", 1, func(args []uintptr) uintptr {
		var x, y float64
		if native.GdkEventGetPosition(args[1], &x, &y) != 0 {
			w.pointerMoved(x, y)
		}
		return 0 // always propagate
	})
	native.GtkWidgetAddController(w.win, ptr)
	return w, nil
}

// pointerMoved converts a pointer position from surface coordinates to
// client-area (GtkFixed) coordinates and re-evaluates input regions.
func (w *window) pointerMoved(sx, sy float64) {
	if len(w.regionViews) == 0 {
		w.ptrKnown = false
		return
	}
	// Surface coords include the CSD shadow margin; the transform is
	// that margin (GTK itself subtracts it in event handling).
	var tx, ty float64
	native.GtkNativeGetSurfaceTransform(native.GtkWidgetGetNative(w.win), &tx, &ty)
	var fx, fy float64
	if native.GtkWidgetTranslateCoordinates(w.win, w.fixed, sx-tx, sy-ty, &fx, &fy) == 0 {
		return
	}
	w.ptrX, w.ptrY, w.ptrKnown = fx, fy, true
	w.retarget()
}

// retarget applies each region-limited view's can-target for the current
// pointer position. Regions are physical px in client-area coordinates.
func (w *window) retarget() {
	scale := float64(max(int(native.GtkWidgetGetScaleFactor(w.win)), 1))
	px, py := int(w.ptrX*scale), int(w.ptrY*scale)
	for _, v := range w.regionViews {
		hit := false
		if w.ptrKnown {
			for _, r := range v.regions {
				if px >= r.X && px < r.X+r.W && py >= r.Y && py < r.Y+r.H {
					hit = true
					break
				}
			}
		}
		v.setCanTarget(hit)
	}
}

// restack keeps overlay views above everything else; called after any
// view is added to the fixed. GtkFixed renders children in sibling
// order and picks topmost-first.
func (w *window) restack() {
	for _, v := range w.overlays {
		native.GtkWidgetInsertBefore(v.view, w.fixed, 0)
	}
}

// dropView forgets a closing view from the stacking and region lists.
func (w *window) dropView(v *webView) {
	w.overlays = remove(w.overlays, v)
	w.regionViews = remove(w.regionViews, v)
}

func remove(s []*webView, v *webView) []*webView {
	for i, x := range s {
		if x == v {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
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
	cw := int(native.GtkWidgetGetWidth(w.clip)) * scale
	ch := int(native.GtkWidgetGetHeight(w.clip)) * scale
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
