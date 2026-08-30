// Package spectacle is a small shell for webview desktop apps: one native
// window, any number of embedded OS webviews with isolated browsing
// profiles, and a tray icon with notifications. Windows (Win32 + WebView2)
// and Linux (GTK4 + WebKitGTK 6.0, bound via purego) are supported; both
// backends are pure Go, no cgo.
//
// Applications are written against the Backend, Window, WebView and Tray
// interfaces below and obtain a Backend from New. The entire UI lives on
// one OS-locked thread; Dispatch is the only door in from other goroutines.
package spectacle

import "image"

// Rect is a rectangle in physical pixels, relative to the window's client
// area origin.
type Rect struct {
	X, Y, W, H int
}

// MenuItem is one entry in a tray menu. A zero Label renders a separator.
type MenuItem struct {
	Label   string
	OnClick func()
}

// webViewConfig collects NewWebView options.
type webViewConfig struct {
	overlay bool
}

// WebViewOption configures a webview at creation.
type WebViewOption func(*webViewConfig)

// Overlay creates the webview as a transparent overlay: it stacks above
// every non-overlay webview in the window regardless of creation order,
// and its background is transparent, so the page decides which pixels
// exist (an unstyled body shows the views underneath). Overlays receive
// pointer input over their whole surface until SetInputRegions narrows
// that down. Check SupportsOverlay first: on backends without overlay
// support, NewWebView rejects the option.
func Overlay() WebViewOption {
	return func(c *webViewConfig) { c.overlay = true }
}

// Backend owns the UI thread, the native event loop, and webview creation.
//
// Every method except Dispatch must be called on the UI thread: either
// before Run (during startup) or from within a callback. Dispatch is the
// only door in from other goroutines.
type Backend interface {
	// NewWindow creates the (hidden) main window.
	NewWindow(title string, bounds Rect) (Window, error)
	// NewWebView creates a webview as a child of w. profile selects an
	// isolated browsing session; webviews sharing a profile share cookies
	// and storage. The empty profile is the default session.
	NewWebView(w Window, profile string, options ...WebViewOption) (WebView, error)
	// SupportsOverlay reports whether this backend can create Overlay
	// webviews (transparent, stacked, input-region-aware).
	SupportsOverlay() bool
	// Run enters the native event loop and blocks until Quit.
	Run() error
	// Dispatch schedules f on the UI thread. Safe from any goroutine.
	Dispatch(f func())
	// OpenURL opens url in the user's default browser.
	OpenURL(url string)
	// Quit ends the event loop and releases native resources.
	Quit()
}

// Window is the native top-level window.
type Window interface {
	SetBounds(Rect)
	Bounds() Rect
	Show()
	Hide()
	IsVisible() bool
	// OnResize reports the client area size in physical pixels and the
	// current DPI whenever either changes.
	OnResize(func(w, h, dpi int))
	// OnCloseRequest is consulted when the user asks to close the window.
	// Returning false vetoes the close (the app may hide to tray instead).
	OnCloseRequest(func() bool)
	Tray() Tray
}

// WebView is one embedded browser view inside a Window.
type WebView interface {
	Navigate(url string)
	// NavigateHTML renders the given document directly, without a server.
	NavigateHTML(html string)
	// InitScript registers js to run in every document before it loads.
	// Must be called before the first navigation to take effect there.
	InitScript(js string)
	// PostJSON delivers a JSON document to the page; the page receives it
	// via its message event. OnMessage receives JSON the page posts back.
	PostJSON(json string)
	OnMessage(func(json string))
	OnTitleChanged(func(title string))
	// OnFaviconChanged delivers the page's favicon as PNG bytes whenever
	// it changes. Backends without favicon support may never call it.
	OnFaviconChanged(func(png []byte))
	// OnNewWindow consults fn for every popup the page opens. Returning
	// true (the URL was handled elsewhere) suppresses the popup; false
	// lets the backend open its default popup window. fn must decide
	// synchronously.
	OnNewWindow(func(url string) bool)
	// OnNotification subscribes to web notifications raised by the page —
	// service workers included — and reports whether the backend supports
	// that. When it returns false, the caller must provide its own
	// notification capture (script shim).
	OnNotification(func(title, body string)) bool
	// SetMemoryTargetLow trims the view's memory when hidden; script
	// keeps running. Best-effort.
	SetMemoryTargetLow(low bool)
	// Suspend stops the view entirely (script included) until Resume.
	// Only for views with no background duties. Best-effort.
	Suspend()
	Resume()
	SetBounds(Rect)
	SetVisible(bool)
	Focus()
	// SetInputRegions restricts where this webview receives pointer
	// input, in physical pixels relative to the window's client area.
	// Outside the regions the pointer falls through to whatever is
	// underneath. nil restores the default (input everywhere); an empty
	// non-nil slice makes the whole view click-through. The view keeps
	// rendering everywhere regardless — regions gate input, not pixels.
	// Best-effort: backends without overlay support ignore it.
	SetInputRegions([]Rect)
	// DeleteProfile marks this webview's browsing profile (cookies,
	// storage) for removal from disk; it takes effect when the profile's
	// webviews close. Best-effort — unsupported backends keep the data.
	DeleteProfile()
	// Close releases the webview and flushes its session to disk.
	Close()
}

// Tray is the system notification-area icon.
type Tray interface {
	// Available reports whether the icon is actually presented somewhere
	// the user can reach (some Linux desktops have no tray host). When
	// false, hide-to-tray would strand the app.
	Available() bool
	SetTooltip(string)
	SetMenu([]MenuItem)
	// Notify shows a desktop notification attributed to the app. icon,
	// when non-nil, is shown in place of the app icon.
	Notify(title, body string, icon image.Image)
	// OnActivate fires when the user clicks the tray icon.
	OnActivate(func())
	// OnNotificationClick fires when the user clicks a notification
	// shown via Notify. Falls back to OnActivate when unset.
	OnNotificationClick(func())
}
