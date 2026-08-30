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
	ctrl   *webview2.Controller
	core   *webview2.CoreWebView2
	closed bool
}

var webViewCount int

func newWebView(w *window, ctrl *webview2.Controller, debug bool) (*webView, error) {
	core, err := ctrl.CoreWebView2()
	if err != nil {
		return nil, err
	}
	webViewCount++
	v := &webView{id: webViewCount, ctrl: ctrl, core: core}
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
	err := v.ctrl.SetBounds(w32.Rect{
		Left:   int32(r.X),
		Top:    int32(r.Y),
		Right:  int32(r.X + r.W),
		Bottom: int32(r.Y + r.H),
	})
	if err != nil {
		log.Printf("win: %v", err)
	}
}

func (v *webView) SetVisible(visible bool) {
	if v.closed {
		return
	}
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

// SetInputRegions is a no-op until this backend supports overlays
// (per contract: best-effort).
func (v *webView) SetInputRegions([]Rect) {}

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
	v.ctrl.Close()
}
