//go:build linux

package spectacle

import (
	_ "embed"

	"github.com/pietjan/spectacle/internal/native"
)

// defaultUserAgent avoids "this browser may not be secure" walls (Google OAuth
// rejects the stock WebKitGTK identity).
const defaultUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.4 Safari/605.1.15"

// polyfill emulates WebView2's window.chrome.webview over WebKit script
// messaging, so every page, shim, and glue script runs byte-identical
// on both platforms. Added before any InitScript.
//
//go:embed polyfill.js
var polyfill string

// webView is one WebKitWebView inside the window's GtkFixed.
type webView struct {
	backend *backend
	win     *window
	view    uintptr // WebKitWebView*
	ucm     uintptr // WebKitUserContentManager*
	profile string
	overlay bool
	closed  bool

	regions   []Rect // non-nil: input limited to these (see SetInputRegions)
	canTarget bool
}

func newWebView(b *backend, w *window, profile string, cfg webViewConfig) (*webView, error) {
	sess := b.sessions.get(profile)
	ucm := native.WebkitUserContentManagerNew()
	view := native.ObjectNew(native.WebkitWebViewGetType(), []native.Prop{
		{Name: "network-session", Object: sess.handle},
		{Name: "user-content-manager", Object: ucm},
	})
	native.GObjectRefSink(view)
	v := &webView{backend: b, win: w, view: view, ucm: ucm, profile: profile,
		overlay: cfg.overlay, canTarget: true}

	// New webviews start visible: the platform contract.
	native.GtkFixedPut(w.fixed, view, 0, 0)
	native.GtkWidgetSetVisible(view, 1)
	if v.overlay {
		w.overlays = append(w.overlays, v)
	}
	w.restack()

	// Bridge plumbing before anything can navigate: the message handler
	// and the WebView2-compat polyfill (added first, so later
	// InitScripts can rely on window.chrome.webview existing).
	native.WebkitUserContentManagerRegisterScriptMessageHandler(ucm, "spectacle", 0)
	v.addUserScript(polyfill)

	settings := native.WebkitWebViewGetSettings(view)
	ua := conf.UserAgent
	if ua == "" {
		ua = defaultUserAgent
	}
	native.WebkitSettingsSetUserAgent(settings, ua)
	if b.debug {
		native.WebkitSettingsSetEnableDeveloperExtras(settings, 1)
		native.WebkitSettingsSetEnableWriteConsoleMessagesToStdout(settings, 1)
	}
	if v.overlay {
		// The page paints only what it wants visible; everything else
		// shows the views underneath.
		native.WebkitWebViewSetBackgroundColor(view, &native.GdkRGBA{})
	} else if Dark() {
		native.WebkitWebViewSetBackgroundColor(view, &native.GdkRGBA{
			R: 0x18 / 255.0, G: 0x18 / 255.0, B: 0x1b / 255.0, A: 1, // zinc-900
		})
	}
	// The AutoGrantNotifications analogue.
	native.Connect(view, "permission-request", 1, func(args []uintptr) uintptr {
		req := args[1]
		if native.GTypeCheckInstanceIsA(req, native.WebkitNotificationPermissionRequestGetType()) != 0 {
			native.WebkitPermissionRequestAllow(req)
			return 1
		}
		return 0
	})
	return v, nil
}

func (v *webView) addUserScript(js string) {
	script := native.WebkitUserScriptNew(js,
		native.UserContentInjectAllFrames, native.UserScriptInjectAtDocumentStart, 0, 0)
	native.WebkitUserContentManagerAddScript(v.ucm, script)
	native.WebkitUserScriptUnref(script)
}

func (v *webView) Navigate(url string) {
	if v.closed {
		return
	}
	native.WebkitWebViewLoadURI(v.view, url)
}

func (v *webView) NavigateHTML(html string) {
	if v.closed {
		return
	}
	native.WebkitWebViewLoadHTML(v.view, html, 0)
}

func (v *webView) InitScript(js string) {
	if v.closed {
		return
	}
	v.addUserScript(js)
}

func (v *webView) PostJSON(json string) {
	if v.closed {
		return
	}
	// json is a complete JSON document — a JS expression subset, safe
	// to inline. Optional chaining guards a page mid-navigation.
	script := "window.chrome?.webview?.__deliver?.(" + json + ")"
	native.WebkitWebViewEvaluateJavascript(v.view, script, -1, 0, 0, 0, 0, 0)
}

func (v *webView) OnMessage(fn func(json string)) {
	native.Connect(v.ucm, "script-message-received::spectacle", 1, func(args []uintptr) uintptr {
		fn(native.JSCValueString(args[1]))
		return 0
	})
}

func (v *webView) OnTitleChanged(fn func(title string)) {
	native.Connect(v.view, "notify::title", 1, func([]uintptr) uintptr {
		fn(native.GoString(native.WebkitWebViewGetTitle(v.view)))
		return 0
	})
}

func (v *webView) OnFaviconChanged(fn func(png []byte)) {
	native.Connect(v.view, "notify::favicon", 1, func([]uintptr) uintptr {
		texture := native.WebkitWebViewGetFavicon(v.view)
		if texture == 0 {
			return 0
		}
		bytes := native.GdkTextureSaveToPngBytes(texture)
		if bytes == 0 {
			return 0
		}
		png := native.BytesCopy(bytes)
		native.GBytesUnref(bytes)
		if len(png) > 0 {
			fn(png)
		}
		return 0
	})
}

func (v *webView) OnNotification(fn func(title, body string)) bool {
	native.Connect(v.view, "show-notification", 1, func(args []uintptr) uintptr {
		n := args[1]
		fn(native.GoString(native.WebkitNotificationGetTitle(n)),
			native.GoString(native.WebkitNotificationGetBody(n)))
		return 1 // handled: no double presentation
	})
	return true
}

func (v *webView) OnNewWindow(fn func(url string) bool) {
	native.Connect(v.view, "create", 1, func(args []uintptr) uintptr {
		req := native.WebkitNavigationActionGetRequest(args[1])
		url := native.GoString(native.WebkitURIRequestGetURI(req))
		// The decision is synchronous on the signal stack, per contract.
		if fn(url) {
			return 0 // handled externally; no popup
		}
		return v.newPopup()
	})
}

// newPopup builds the default popup window: a related webview (same web
// process, same session — OAuth flows depend on that) in its own window.
func (v *webView) newPopup() uintptr {
	popup := native.ObjectNew(native.WebkitWebViewGetType(), []native.Prop{
		{Name: "related-view", Object: v.view},
	})
	win := native.GtkWindowNew()
	native.GtkWindowSetTitle(win, conf.Name)
	native.GtkWindowSetDefaultSize(win, 600, 700)
	native.GtkWindowSetChild(win, popup)
	native.Connect(popup, "close", 0, func([]uintptr) uintptr {
		native.GtkWindowDestroy(win)
		return 0
	})
	native.GtkWindowPresent(win)
	return popup
}

func (v *webView) SetBounds(r Rect) {
	if v.closed {
		return
	}
	scale := max(int(native.GtkWidgetGetScaleFactor(v.win.fixed)), 1)
	native.GtkFixedMove(v.win.fixed, v.view, float64(r.X/scale), float64(r.Y/scale))
	native.GtkWidgetSetSizeRequest(v.view, int32(r.W/scale), int32(r.H/scale))
}

func (v *webView) SetVisible(visible bool) {
	if v.closed {
		return
	}
	b := int32(0)
	if visible {
		b = 1
	}
	native.GtkWidgetSetVisible(v.view, b)
}

func (v *webView) Focus() {
	if v.closed {
		return
	}
	native.GtkWidgetGrabFocus(v.view)
}

// SetInputRegions limits pointer input to regions (physical px, client
// area coordinates). nil restores input everywhere; empty non-nil makes
// the view fully click-through. Implemented by flipping can-target from
// pointer tracking on the window — GTK4 has no per-region widget input.
func (v *webView) SetInputRegions(regions []Rect) {
	if v.closed {
		return
	}
	if regions == nil {
		if v.regions != nil {
			v.regions = nil
			v.win.regionViews = remove(v.win.regionViews, v)
			v.setCanTarget(true)
		}
		return
	}
	if v.regions == nil {
		v.win.regionViews = append(v.win.regionViews, v)
	}
	v.regions = append([]Rect(nil), regions...)
	// Re-evaluate immediately: regions can change under a resting
	// pointer (e.g. a modal opened from the keyboard).
	v.win.retarget()
}

func (v *webView) setCanTarget(t bool) {
	if v.canTarget == t {
		return
	}
	v.canTarget = t
	b := int32(0)
	if t {
		b = 1
	}
	native.GtkWidgetSetCanTarget(v.view, b)
}

// Best-effort no-ops on this backend.
func (v *webView) SetMemoryTargetLow(bool) {}
func (v *webView) Suspend()                {}
func (v *webView) Resume()                 {}

func (v *webView) DeleteProfile() {
	if v.closed {
		return
	}
	v.backend.sessions.doom(v.profile)
}

func (v *webView) Close() {
	if v.closed {
		return
	}
	v.closed = true
	v.win.dropView(v)
	native.WebkitWebViewTryClose(v.view)
	native.GtkFixedRemove(v.win.fixed, v.view)
	native.GObjectUnref(v.view)
	native.GObjectUnref(v.ucm)
	v.backend.sessions.release(v.profile)
}
