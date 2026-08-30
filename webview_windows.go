//go:build windows

package spectacle

import (
	"log"
	"time"

	"github.com/pietjan/spectacle/internal/w32"
	"github.com/pietjan/spectacle/internal/webview2"
)

// WebView adapts a WebView2 controller pair to WebView.
type webView struct {
	id     int
	win    *window
	ctrl   *webview2.Controller
	core   *webview2.CoreWebView2
	closed bool

	// Composition hosting (nil comp/visual on the windowed fallback).
	comp    *webview2.CompositionController
	visual  *webview2.Visual
	overlay bool
	bounds  Rect
	regions []Rect
	visible bool
	cursor  uintptr
}

var webViewCount int

func newWebView(w *window, ctrl *webview2.Controller, debug bool) (*webView, error) {
	core, err := ctrl.CoreWebView2()
	if err != nil {
		return nil, err
	}
	webViewCount++
	v := &webView{id: webViewCount, win: w, ctrl: ctrl, core: core, visible: true}
	cursor, _, _ := w32.LoadCursor.Call(0, uintptr(w32.IdcArrow))
	v.cursor = cursor
	// Controllers do not reliably start visible; make "new webview is
	// visible" the platform contract and let the app hide what it hides.
	if err := ctrl.SetVisible(true); err != nil {
		return nil, err
	}
	// Backdrop matching the theme, so loads and switches never flash white.
	if osAppsUseDarkTheme() {
		if err := ctrl.SetDefaultBackgroundColor(0x18, 0x18, 0x1b); err != nil { // zinc-900
			log.Printf("win: %v", err)
		}
	}
	if err := core.ConfigureSettings(debug); err != nil {
		return nil, err
	}
	// Steady-state diagnostics: what the controller actually reports,
	// once startup has settled.
	if debug {
		id := v.id
		time.AfterFunc(5*time.Second, func() {
			w.backend.Dispatch(func() {
				if v.closed {
					return
				}
				b, err := ctrl.Bounds()
				log.Printf("win: webview %d steady: bounds=%+v visible=%v err=%v", id, b, ctrl.IsVisible(), err)
				core.ExecuteScript(`innerWidth+"x"+innerHeight+" "+location.href.slice(0,40)+" Notification="+Notification.name+"/"+Notification.permission`,
					func(r string) { log.Printf("win: webview %d page: %s", id, r) })
			})
		})
	}
	if err := core.AutoGrantNotifications(); err != nil {
		return nil, err
	}
	w.onMove = append(w.onMove, func() {
		// The hook outlives the webview; never call into a closed one.
		if !v.closed {
			ctrl.NotifyParentWindowPositionChanged()
		}
	})
	return v, nil
}

func (v *webView) Navigate(url string) {
	if v.closed {
		return
	}
	if err := v.core.Navigate(url); err != nil {
		log.Printf("win: %v", err)
	}
}

func (v *webView) NavigateHTML(html string) {
	if v.closed {
		return
	}
	if err := v.core.NavigateToString(html); err != nil {
		log.Printf("win: %v", err)
	}
}

func (v *webView) InitScript(js string) {
	if v.closed {
		return
	}
	if err := v.core.AddScriptOnDocumentCreated(js); err != nil {
		log.Printf("win: %v", err)
	}
}

func (v *webView) PostJSON(json string) {
	if v.closed {
		return
	}
	if err := v.core.PostWebMessageAsJson(json); err != nil {
		log.Printf("win: %v", err)
	}
}

func (v *webView) OnMessage(fn func(json string)) {
	if err := v.core.OnWebMessageReceived(fn); err != nil {
		log.Printf("win: %v", err)
	}
}

func (v *webView) OnTitleChanged(fn func(title string)) {
	if err := v.core.OnDocumentTitleChanged(fn); err != nil {
		log.Printf("win: %v", err)
	}
}

func (v *webView) OnFaviconChanged(fn func(png []byte)) {
	if err := v.core.OnFaviconChanged(fn); err != nil {
		// Pre-2023 runtime: icons degrade to initials, nothing broken.
		log.Printf("win: favicons unavailable: %v", err)
	}
}

func (v *webView) OnNewWindow(fn func(url string) bool) {
	if err := v.core.OnNewWindowRequested(fn); err != nil {
		log.Printf("win: %v", err)
	}
}

func (v *webView) OnNotification(fn func(title, body string)) bool {
	if err := v.core.OnNotificationReceived(fn); err != nil {
		// Pre-2024 runtime: the caller falls back to the script shim.
		log.Printf("win: native notifications unavailable: %v", err)
		return false
	}
	return true
}

func (v *webView) SetMemoryTargetLow(low bool) {
	if v.closed {
		return
	}
	if err := v.core.SetMemoryTargetLow(low); err != nil {
		log.Printf("win: memory target: %v", err)
	}
}

func (v *webView) Suspend() {
	if v.closed {
		return
	}
	if err := v.core.TrySuspend(); err != nil {
		log.Printf("win: suspend: %v", err)
	}
}

func (v *webView) Resume() {
	if v.closed {
		return
	}
	if err := v.core.Resume(); err != nil {
		log.Printf("win: resume: %v", err)
	}
}

func (v *webView) SetBounds(r Rect) {
	if v.closed {
		return
	}
	v.bounds = r
	err := v.ctrl.SetBounds(w32.Rect{
		Left:   int32(r.X),
		Top:    int32(r.Y),
		Right:  int32(r.X + r.W),
		Bottom: int32(r.Y + r.H),
	})
	if err != nil {
		log.Printf("win: %v", err)
	}
	if v.comp != nil {
		// Visual hosting renders at the visual's origin regardless of
		// the bounds' left/top; position comes from the visual.
		if err := v.visual.SetTranslation(int32(r.X), int32(r.Y)); err != nil {
			log.Printf("win: %v", err)
		}
		if v.win.backend.dcomp != nil {
			v.win.backend.dcomp.Commit()
		}
	}
}

func (v *webView) SetVisible(visible bool) {
	if v.closed {
		return
	}
	v.visible = visible
	if err := v.ctrl.SetVisible(visible); err != nil {
		log.Printf("win: %v", err)
	}
}

func (v *webView) Focus() {
	if v.closed {
		return
	}
	if err := v.ctrl.MoveFocus(); err != nil {
		log.Printf("win: %v", err)
	}
}

// SetInputRegions limits pointer input to regions (window client
// coordinates); the window's mouse router consults them on every hit
// test. No-op on the windowed fallback (per contract: best-effort).
func (v *webView) SetInputRegions(regions []Rect) {
	if v.closed || v.comp == nil {
		return
	}
	if regions == nil {
		v.regions = nil
		return
	}
	v.regions = append([]Rect(nil), regions...)
	// The pointer may be sitting inside a region that just changed;
	// resync enter/leave on the next real mouse event (cheap and lazy).
}

// sendMouse forwards one mouse event to a composition-hosted view.
func (v *webView) sendMouse(kind, keys, data uint32, x, y int32) {
	if v.closed || v.comp == nil {
		return
	}
	if err := v.comp.SendMouseInput(kind, keys, data, x, y); err != nil {
		log.Printf("win: %v", err)
	}
}

func (v *webView) DeleteProfile() {
	if v.closed {
		return
	}
	if err := v.core.DeleteProfile(); err != nil {
		log.Printf("win: delete profile: %v", err)
	}
}

func (v *webView) Close() {
	if v.closed {
		return
	}
	v.closed = true
	if v.comp != nil {
		v.win.detachView(v)
	}
	v.ctrl.Close()
	if v.comp != nil {
		v.comp.Release()
		v.visual.Release()
	}
}
