//go:build windows

package spectacle

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/registry"

	"github.com/pietjan/spectacle/internal/w32"
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

	tray *tray
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
	}
	r, _, _ := w32.DefWindowProc.Call(w.hwnd, msg, wparam, uintptr(lparam))
	return r
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
