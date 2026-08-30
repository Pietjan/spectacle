//go:build windows

package spectacle

import (
	"image"
	"log"
	"unsafe"

	xdraw "golang.org/x/image/draw"

	"github.com/pietjan/spectacle/internal/w32"
)

const trayIconID = 1

// tray is the notification-area icon, its menu, and balloon notifications.
type tray struct {
	win           *window
	menu          []MenuItem
	onActivate    func()
	onNotifyClick func()
	// balloonIcon is the HICON of the last shown notification. The shell
	// renders toasts asynchronously, so it must outlive the Notify call;
	// it is destroyed when the next notification replaces it.
	balloonIcon uintptr
}

func newTray(w *window) *tray {
	t := &tray{win: w}
	data := t.data()
	data.UFlags = w32.NifMessage | w32.NifIcon | w32.NifTip
	data.UCallbackMessage = wmAppTray
	data.HIcon = appIcon()
	w32.ShellNotifyIcon.Call(w32.NimAdd, uintptr(unsafe.Pointer(data)))
	data.UVersion = w32.NotifyIconVersion4
	w32.ShellNotifyIcon.Call(w32.NimSetVersion, uintptr(unsafe.Pointer(data)))
	return t
}

func (t *tray) data() *w32.NotifyIconData {
	d := &w32.NotifyIconData{HWnd: t.win.hwnd, UID: trayIconID}
	d.CbSize = uint32(unsafe.Sizeof(*d))
	return d
}

// Available is always true on Windows: the notification area exists.
func (t *tray) Available() bool { return true }

// SetTooltip sets the hover text.
func (t *tray) SetTooltip(tip string) {
	d := t.data()
	d.UFlags = w32.NifTip
	copyUTF16(d.SzTip[:], tip)
	w32.ShellNotifyIcon.Call(w32.NimModify, uintptr(unsafe.Pointer(d)))
}

// SetMenu sets the right-click menu.
func (t *tray) SetMenu(items []MenuItem) { t.menu = items }

// Notify shows a balloon notification (a native toast on Windows 10/11),
// with the given image as its icon when one decodes.
func (t *tray) Notify(title, body string, icon image.Image) {
	d := t.data()
	d.UFlags = w32.NifInfo
	d.DwInfoFlags = w32.NiifInfo
	copyUTF16(d.SzInfoTitle[:], title)
	copyUTF16(d.SzInfo[:], body)
	var hicon uintptr
	if icon != nil {
		if hicon = iconFromImage(icon); hicon != 0 {
			d.DwInfoFlags = w32.NiifUser | w32.NiifLargeIcon
			d.HBalloonIcon = hicon
		}
	}
	r, _, err := w32.ShellNotifyIcon.Call(w32.NimModify, uintptr(unsafe.Pointer(d)))
	if r == 0 {
		log.Printf("win: tray notify failed: %v", err)
	}
	if t.balloonIcon != 0 {
		w32.DestroyIcon.Call(t.balloonIcon)
	}
	t.balloonIcon = hicon
}

// notifyIconSize is the balloon icon edge in pixels — generous, since
// toasts render it at ~48 logical pixels and the shell downscales well.
const notifyIconSize = 64

// iconFromImage converts an image to a 32bpp ARGB HICON, resampled to
// notifyIconSize. Returns 0 on failure.
func iconFromImage(src image.Image) uintptr {
	const n = notifyIconSize
	b := src.Bounds()
	if b.Dx() == 0 || b.Dy() == 0 {
		return 0
	}
	scaled := image.NewRGBA(image.Rect(0, 0, n, n))
	xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), src, b, xdraw.Over, nil)
	// BGRA, premultiplied, top-down — what CreateBitmap expects for a
	// 32bpp DDB used as an alpha icon. image.RGBA is premultiplied.
	bits := make([]byte, n*n*4)
	for i := 0; i < n*n; i++ {
		bits[i*4+0] = scaled.Pix[i*4+2]
		bits[i*4+1] = scaled.Pix[i*4+1]
		bits[i*4+2] = scaled.Pix[i*4+0]
		bits[i*4+3] = scaled.Pix[i*4+3]
	}
	color, _, _ := w32.CreateBitmap.Call(n, n, 1, 32, uintptr(unsafe.Pointer(&bits[0])))
	if color == 0 {
		return 0
	}
	defer w32.DeleteObject.Call(color)
	mask := make([]byte, n*n/8)
	mono, _, _ := w32.CreateBitmap.Call(n, n, 1, 1, uintptr(unsafe.Pointer(&mask[0])))
	if mono == 0 {
		return 0
	}
	defer w32.DeleteObject.Call(mono)
	info := w32.IconInfo{FIcon: 1, HbmMask: mono, HbmColor: color}
	hicon, _, _ := w32.CreateIconIndirect.Call(uintptr(unsafe.Pointer(&info)))
	return hicon
}

// OnActivate registers the icon-click handler.
func (t *tray) OnActivate(fn func()) { t.onActivate = fn }

// OnNotificationClick registers the notification-click handler.
func (t *tray) OnNotificationClick(fn func()) { t.onNotifyClick = fn }

// onEvent handles the tray callback message (version-4 encoding: the
// event lives in LOWORD(lParam)).
func (t *tray) onEvent(event uint16) {
	const wmContextMenu = 0x007B
	switch event {
	case w32.WmLButtonUp:
		if t.onActivate != nil {
			t.onActivate()
		}
	case w32.NinBalloonUserClick:
		switch {
		case t.onNotifyClick != nil:
			t.onNotifyClick()
		case t.onActivate != nil:
			t.onActivate()
		}
	case w32.WmRButtonUp, wmContextMenu:
		t.showMenu()
	}
}

func (t *tray) showMenu() {
	if len(t.menu) == 0 {
		return
	}
	menu, _, _ := w32.CreatePopupMenu.Call()
	defer w32.DestroyMenu.Call(menu)
	for i, item := range t.menu {
		if item.Label == "" {
			w32.AppendMenu.Call(menu, w32.MfSeparator, 0, 0)
			continue
		}
		w32.AppendMenu.Call(menu, w32.MfString, uintptr(i+1),
			uintptr(unsafe.Pointer(utf16Ptr(item.Label))))
	}
	// Required quirk: without foregrounding the window first, the menu
	// won't dismiss when the user clicks away.
	w32.SetForegroundWindow.Call(t.win.hwnd)
	var pt w32.Point
	w32.GetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	cmd, _, _ := w32.TrackPopupMenu.Call(menu,
		w32.TpmReturnCmd|w32.TpmRightButton,
		uintptr(pt.X), uintptr(pt.Y), 0, t.win.hwnd, 0)
	if cmd == 0 {
		return
	}
	item := t.menu[cmd-1]
	if item.OnClick != nil {
		item.OnClick()
	}
}

// remove deletes the icon; call on shutdown.
func (t *tray) remove() {
	w32.ShellNotifyIcon.Call(w32.NimDelete, uintptr(unsafe.Pointer(t.data())))
	if t.balloonIcon != 0 {
		w32.DestroyIcon.Call(t.balloonIcon)
		t.balloonIcon = 0
	}
}

func copyUTF16(dst []uint16, s string) {
	u, err := utf16FromString(s)
	if err != nil {
		return
	}
	n := copy(dst, u)
	if n == len(dst) {
		dst[len(dst)-1] = 0
	}
}
