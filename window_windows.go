//go:build windows

package spectacle

import (
	"fmt"
	"log"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/registry"

	"github.com/pietjan/spectacle/internal/w32"
	"github.com/pietjan/spectacle/internal/webview2"
)

const (
	wmAppTray     = w32.WmApp + 1
	wmAppDispatch = w32.WmApp + 2
)

func className() string { return conf.ID + "-main" }

// window is the Win32 top-level window.
type window struct {
	backend *backend
	hwnd    uintptr

	onResize func(w, h, dpi int)
	onClose  func() bool
	onMove   []func()

	// Composition hosting: the visual tree root and the mouse router's
	// state. views is z-ordered bottom→top (overlays kept above normal
	// views); the window proc hit-tests it and forwards mouse input.
	target   *webview2.CompositionTarget
	root     *webview2.Visual
	views    []*webView
	mouseIn  *webView // view under the pointer (enter/leave tracking)
	captured *webView // view holding the implicit drag capture
	focused  *webView
	tracking bool // TrackMouseEvent armed

	// OLE drag-and-drop routing state.
	dragData unsafe.Pointer // IDataObject*, held (AddRef'd) for the drag
	dragView *webView       // view the drag is currently over

	tray *tray
}

// ensureComposition lazily builds the window's composition target and
// root visual.
func (w *window) ensureComposition(dev *webview2.CompositionDevice) error {
	if w.root != nil {
		return nil
	}
	target, err := dev.CreateTargetForHwnd(w.hwnd)
	if err != nil {
		return err
	}
	root, err := dev.CreateVisual()
	if err != nil {
		return err
	}
	if err := target.SetRoot(root); err != nil {
		return err
	}
	w.target, w.root = target, root
	// Composition hosting makes the host window the drop target; route
	// OLE drags to the view under the cursor. Best-effort: without it,
	// drops just do nothing.
	if err := webview2.RegisterDropTarget(w.hwnd, w); err != nil {
		log.Printf("win: drag-and-drop unavailable: %v", err)
	}
	return nil
}

// dropPoint converts an OLE screen coordinate to window client space.
func (w *window) dropPoint(x, y int32) (int32, int32) {
	pt := w32.Point{X: x, Y: y}
	w32.ScreenToClient.Call(w.hwnd, uintptr(unsafe.Pointer(&pt)))
	return pt.X, pt.Y
}

// dragTransition moves the drag between views mid-flight, pairing
// DragLeave/DragEnter per controller the way WebView2 expects.
func (w *window) dragTransition(v *webView, keyState uint32, cx, cy int32, effect *uint32) {
	if v == w.dragView {
		return
	}
	if w.dragView != nil {
		w.dragView.comp.DragLeave()
	}
	w.dragView = v
	if v != nil {
		v.comp.DragEnter(w.dragData, keyState, cx-int32(v.bounds.X), cy-int32(v.bounds.Y), effect)
	}
}

// DragEnter implements webview2.DropRouter.
func (w *window) DragEnter(data unsafe.Pointer, keyState uint32, x, y int32, effect *uint32) {
	w.dragData = data
	webview2.AddRefObject(data)
	cx, cy := w.dropPoint(x, y)
	w.dragTransition(w.hitTest(cx, cy), keyState, cx, cy, effect)
	if w.dragView == nil {
		*effect = 0
	}
}

// DragOver implements webview2.DropRouter.
func (w *window) DragOver(keyState uint32, x, y int32, effect *uint32) {
	cx, cy := w.dropPoint(x, y)
	v := w.hitTest(cx, cy)
	w.dragTransition(v, keyState, cx, cy, effect)
	if v == nil {
		*effect = 0
		return
	}
	v.comp.DragOver(keyState, cx-int32(v.bounds.X), cy-int32(v.bounds.Y), effect)
}

// DragLeave implements webview2.DropRouter.
func (w *window) DragLeave() {
	if w.dragView != nil {
		w.dragView.comp.DragLeave()
		w.dragView = nil
	}
	if w.dragData != nil {
		webview2.ReleaseObject(w.dragData)
		w.dragData = nil
	}
}

// Drop implements webview2.DropRouter.
func (w *window) Drop(data unsafe.Pointer, keyState uint32, x, y int32, effect *uint32) {
	cx, cy := w.dropPoint(x, y)
	v := w.hitTest(cx, cy)
	w.dragTransition(v, keyState, cx, cy, effect)
	if v != nil {
		v.comp.Drop(data, keyState, cx-int32(v.bounds.X), cy-int32(v.bounds.Y), effect)
	} else {
		*effect = 0
	}
	w.dragView = nil
	if w.dragData != nil {
		webview2.ReleaseObject(w.dragData)
		w.dragData = nil
	}
}

// attachView inserts v into the z-order and the visual tree: overlays
// go on top, normal views above other normal views but below overlays.
func (w *window) attachView(v *webView) {
	insert := len(w.views)
	if !v.overlay {
		for i, other := range w.views {
			if other.overlay {
				insert = i
				break
			}
		}
	}
	w.views = append(w.views, nil)
	copy(w.views[insert+1:], w.views[insert:])
	w.views[insert] = v
	w.restackVisuals()
}

// restackVisuals rebuilds the root's children to match w.views order.
// View counts are small; correctness beats cleverness here.
func (w *window) restackVisuals() {
	for _, v := range w.views {
		w.root.RemoveVisual(v.visual)
	}
	for _, v := range w.views {
		w.root.AddVisual(v.visual) // appends topmost
	}
}

// detachView removes a closing view from the router and visual tree.
func (w *window) detachView(v *webView) {
	for i, x := range w.views {
		if x == v {
			w.views = append(w.views[:i], w.views[i+1:]...)
			break
		}
	}
	if w.mouseIn == v {
		w.mouseIn = nil
	}
	if w.captured == v {
		w.captured = nil
		w32.ReleaseCapture.Call()
	}
	if w.focused == v {
		w.focused = nil
	}
	if w.dragView == v {
		w.dragView = nil
	}
	if w.root != nil {
		w.root.RemoveVisual(v.visual)
		if w.backend.dcomp != nil {
			w.backend.dcomp.Commit()
		}
	}
}

// windows maps hwnd → Window for the shared wndproc. Single-threaded
// access (UI thread only), no lock needed.
var (
	windows       = map[uintptr]*window{}
	pendingWindow *window
)

// lparam is typed unsafe.Pointer at the trampoline so messages carrying a
// pointer (WM_DPICHANGED's RECT) need no uintptr round trip; integer
// payloads are read back with uintptr(lparam).
var wndProcCallback = syscall.NewCallback(func(hwnd, msg, wparam uintptr, lparam unsafe.Pointer) uintptr {
	w, ok := windows[hwnd]
	if !ok {
		if pendingWindow == nil {
			r, _, _ := w32.DefWindowProc.Call(hwnd, msg, wparam, uintptr(lparam))
			return r
		}
		// First message during CreateWindowEx: adopt the hwnd.
		w = pendingWindow
		w.hwnd = hwnd
		windows[hwnd] = w
	}
	return w.wndProc(msg, wparam, lparam)
})

func (w *window) wndProc(msg, wparam uintptr, lparam unsafe.Pointer) uintptr {
	switch msg {
	case w32.WmSize:
		w.notifyResize()
		return 0
	case w32.WmMove:
		for _, fn := range w.onMove {
			fn()
		}
		return 0
	case w32.WmDpiChanged:
		// Adopt the suggested rect; the resulting WM_SIZE re-lays-out.
		r := (*w32.Rect)(lparam)
		w32.SetWindowPos.Call(w.hwnd, 0,
			uintptr(r.Left), uintptr(r.Top),
			uintptr(r.Right-r.Left), uintptr(r.Bottom-r.Top),
			w32.SwpNoZOrder|w32.SwpNoActivate)
		return 0
	case w32.WmClose:
		if w.onClose != nil && !w.onClose() {
			return 0 // vetoed: the handler hid the window instead
		}
		w32.DestroyWindow.Call(w.hwnd)
		return 0
	case w32.WmDestroy:
		w32.RevokeDragDrop.Call(w.hwnd)
		delete(windows, w.hwnd)
		w32.PostQuitMessage.Call(0)
		return 0
	case wmAppDispatch:
		w.backend.drainDispatch()
		return 0
	case wmAppTray:
		if w.tray != nil {
			w.tray.onEvent(w32.Loword(uintptr(lparam)))
		}
		return 0
	case w32.WmSetCursor:
		if w.mouseIn != nil && w32.Loword(uintptr(lparam)) == w32.HtClient {
			w32.SetCursor.Call(w.mouseIn.cursor)
			return 1
		}
	case w32.WmMouseLeave:
		w.tracking = false
		if w.mouseIn != nil && w.captured == nil {
			w.mouseIn.sendMouse(w32.WmMouseLeave, 0, 0, 0, 0)
			w.mouseIn = nil
		}
		return 0
	case w32.WmSetFocus:
		if w.focused != nil && !w.focused.closed {
			w.focused.ctrl.MoveFocus()
		}
	default:
		if msg >= w32.WmMouseMove && msg <= w32.WmMouseHWheel {
			if w.routeMouse(msg, wparam, uintptr(lparam)) {
				return 0
			}
		}
	}
	r, _, _ := w32.DefWindowProc.Call(w.hwnd, msg, wparam, uintptr(lparam))
	return r
}

// routeMouse forwards a mouse message to the composition-hosted view
// under the pointer (or the one holding the drag capture). Reports
// whether the message was consumed.
func (w *window) routeMouse(msg, wparam, lparam uintptr) bool {
	if len(w.views) == 0 {
		return false
	}
	x := int32(int16(w32.Loword(lparam)))
	y := int32(int16(w32.Hiword(lparam)))
	if msg == w32.WmMouseWheel || msg == w32.WmMouseHWheel {
		// Wheel messages carry screen coordinates.
		pt := w32.Point{X: x, Y: y}
		w32.ScreenToClient.Call(w.hwnd, uintptr(unsafe.Pointer(&pt)))
		x, y = pt.X, pt.Y
	}

	target := w.captured
	if target == nil {
		target = w.hitTest(x, y)
	}
	// Crossing bookkeeping: tell the old view the pointer left it.
	if w.captured == nil && target != w.mouseIn {
		if w.mouseIn != nil {
			w.mouseIn.sendMouse(w32.WmMouseLeave, 0, 0, 0, 0)
		}
		w.mouseIn = target
	}
	if !w.tracking {
		// Ask for one WM_MOUSELEAVE when the pointer exits the window.
		tme := w32.TrackMouseEventData{DwFlags: w32.TmeLeave, HwndTrack: w.hwnd}
		tme.CbSize = uint32(unsafe.Sizeof(tme))
		w32.TrackMouseEvent.Call(uintptr(unsafe.Pointer(&tme)))
		w.tracking = true
	}
	if target == nil {
		return false
	}

	keys := uint32(w32.Loword(wparam))
	var data uint32
	switch msg {
	case w32.WmMouseWheel, w32.WmMouseHWheel:
		data = uint32(int32(int16(w32.Hiword(wparam)))) // signed wheel delta
	case w32.WmXButtonDown, w32.WmXButtonUp, w32.WmXButtonDblClk:
		data = uint32(w32.Hiword(wparam))
	}

	switch msg {
	case w32.WmLButtonDown, w32.WmRButtonDown, w32.WmMButtonDown, w32.WmXButtonDown:
		if w.captured == nil {
			w32.SetCapture.Call(w.hwnd)
			w.captured = target
		}
		if w.focused != target {
			w.focused = target
			target.ctrl.MoveFocus()
		}
	case w32.WmLButtonUp, w32.WmRButtonUp, w32.WmMButtonUp, w32.WmXButtonUp:
		const anyButton = 0x1 | 0x2 | 0x10 | 0x20 | 0x40 // MK_LBUTTON..MK_XBUTTON2
		if w.captured != nil && keys&anyButton == 0 {
			w.captured = nil
			w32.ReleaseCapture.Call()
		}
	}

	target.sendMouse(uint32(msg), keys, data,
		x-int32(target.bounds.X), y-int32(target.bounds.Y))
	return true
}

// hitTest finds the topmost visible view whose bounds (and input
// regions, when set) contain the window-client point.
func (w *window) hitTest(x, y int32) *webView {
	px, py := int(x), int(y)
	for i := len(w.views) - 1; i >= 0; i-- {
		v := w.views[i]
		if v.closed || !v.visible {
			continue
		}
		b := v.bounds
		if px < b.X || px >= b.X+b.W || py < b.Y || py >= b.Y+b.H {
			continue
		}
		if v.regions != nil && !regionsContain(v.regions, px, py) {
			continue
		}
		return v
	}
	return nil
}

func regionsContain(regions []Rect, x, y int) bool {
	for _, r := range regions {
		if x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H {
			return true
		}
	}
	return false
}

func (w *window) notifyResize() {
	if w.onResize == nil {
		return
	}
	var rect w32.Rect
	w32.GetClientRect.Call(w.hwnd, uintptr(unsafe.Pointer(&rect)))
	dpi, _, _ := w32.GetDpiForWindow.Call(w.hwnd)
	w.onResize(int(rect.Right-rect.Left), int(rect.Bottom-rect.Top), int(dpi))
}

func newWindow(b *backend, title string, bounds Rect) (*window, error) {
	hinstance, _, _ := w32.GetModuleHandle.Call(0)
	cursor, _, _ := w32.LoadCursor.Call(0, uintptr(w32.IdcArrow))
	icon := appIcon()

	// The class brush paints exposed client area (during resizes, before
	// webviews cover it); match it to the theme so nothing flashes white.
	background := uintptr(w32.ColorWindow + 1)
	if osAppsUseDarkTheme() {
		if brush, _, _ := w32.CreateSolidBrush.Call(0x001b1818); brush != 0 { // zinc-900, BGR
			background = brush
		}
	}
	cls := w32.WndClassEx{
		Style:         0,
		LpfnWndProc:   wndProcCallback,
		HInstance:     hinstance,
		HIcon:         icon,
		HCursor:       cursor,
		HbrBackground: background,
		LpszClassName: utf16Ptr(className()),
	}
	cls.CbSize = uint32(unsafe.Sizeof(cls))
	if atom, _, err := w32.RegisterClassEx.Call(uintptr(unsafe.Pointer(&cls))); atom == 0 {
		return nil, fmt.Errorf("win: RegisterClassEx: %w", err)
	}

	w := &window{backend: b}
	pendingWindow = w
	hwnd, _, err := w32.CreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(utf16Ptr(className()))),
		uintptr(unsafe.Pointer(utf16Ptr(title))),
		w32.WsOverlappedWindow,
		uintptr(int32(bounds.X)), uintptr(int32(bounds.Y)),
		uintptr(int32(bounds.W)), uintptr(int32(bounds.H)),
		0, 0, hinstance, 0)
	pendingWindow = nil
	if hwnd == 0 {
		return nil, fmt.Errorf("win: CreateWindowEx: %w", err)
	}
	styleTitlebar(hwnd)
	return w, nil
}

// appIcon loads the icon embedded as resource "APP" (winres), falling
// back to the stock application icon when absent (go test binaries).
func appIcon() uintptr {
	hinstance, _, _ := w32.GetModuleHandle.Call(0)
	if icon, _, _ := w32.LoadIcon.Call(hinstance, uintptr(unsafe.Pointer(utf16Ptr("APP")))); icon != 0 {
		return icon
	}
	icon, _, _ := w32.LoadIcon.Call(0, uintptr(w32.IdiApplication))
	return icon
}

// styleTitlebar matches the native titlebar to the app theme: in OS dark
// mode it goes immersive-dark and is painted the sidebar's background
// (zinc-900) with matching border and text. Attributes unsupported by
// the OS (caption color needs Windows 11) fail silently — Windows 10
// still gets the plain dark titlebar.
func styleTitlebar(hwnd uintptr) {
	if !osAppsUseDarkTheme() {
		return
	}
	set := func(attr uintptr, value uint32) {
		w32.DwmSetWindowAttribute.Call(hwnd, attr,
			uintptr(unsafe.Pointer(&value)), unsafe.Sizeof(value))
	}
	set(w32.DwmwaUseImmersiveDarkMode, 1)
	const (
		captionBGR = 0x001b1818 // zinc-900 #18181b
		textBGR    = 0x00f5f4f4 // zinc-100 #f4f4f5
	)
	set(w32.DwmwaCaptionColor, captionBGR)
	set(w32.DwmwaBorderColor, captionBGR)
	set(w32.DwmwaTextColor, textBGR)
}

// osAppsUseDarkTheme reads the per-user app theme choice.
func osAppsUseDarkTheme() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	light, _, err := k.GetIntegerValue("AppsUseLightTheme")
	return err == nil && light == 0
}

// SetBounds moves/resizes the window (outer frame, screen coordinates).
func (w *window) SetBounds(r Rect) {
	w32.SetWindowPos.Call(w.hwnd, 0,
		uintptr(int32(r.X)), uintptr(int32(r.Y)),
		uintptr(int32(r.W)), uintptr(int32(r.H)),
		w32.SwpNoZOrder|w32.SwpNoActivate)
}

// Bounds returns the outer frame rectangle in screen coordinates.
func (w *window) Bounds() Rect {
	var r w32.Rect
	w32.GetWindowRect.Call(w.hwnd, uintptr(unsafe.Pointer(&r)))
	return Rect{X: int(r.Left), Y: int(r.Top), W: int(r.Right - r.Left), H: int(r.Bottom - r.Top)}
}

// Show shows and foregrounds the window.
func (w *window) Show() {
	w32.ShowWindow.Call(w.hwnd, w32.SwShow)
	w32.SetForegroundWindow.Call(w.hwnd)
}

// Hide hides the window (it keeps running; the tray brings it back).
func (w *window) Hide() {
	w32.ShowWindow.Call(w.hwnd, w32.SwHide)
}

// IsVisible reports whether the window is shown.
func (w *window) IsVisible() bool {
	r, _, _ := w32.IsWindowVisible.Call(w.hwnd)
	return r != 0
}

// OnResize registers the resize callback and fires it once with the
// current size so layout starts correct.
func (w *window) OnResize(fn func(width, height, dpi int)) {
	w.onResize = fn
	w.notifyResize()
}

// OnCloseRequest registers the close-veto callback.
func (w *window) OnCloseRequest(fn func() bool) { w.onClose = fn }

// Tray returns (creating on first use) the notification-area icon.
func (w *window) Tray() Tray {
	if w.tray == nil {
		w.tray = newTray(w)
	}
	return w.tray
}

func utf16Ptr(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		panic(err)
	}
	return p
}
