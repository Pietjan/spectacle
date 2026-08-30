# spectacle

A small shell for webview desktop apps in pure Go: one native window, any
number of embedded OS webviews with isolated browsing profiles, and a tray
icon with desktop notifications. No cgo, no Electron, no bundled browser.

- **Windows:** Win32 + WebView2, driven through hand-written pure-Go COM
  bindings (`internal/webview2`).
- **Linux:** GTK4 + WebKitGTK 6.0, bound at runtime via
  [purego](https://github.com/ebitengine/purego) dlopen bindings
  (`internal/native`). Needs only the runtime libraries
  (`libgtk-4-1 libwebkitgtk-6.0-4` on Debian/Ubuntu); tray via
  StatusNotifierItem, notifications via org.freedesktop.Notifications.
- A WKWebView backend for macOS can follow.

Applications are written once against the platform-neutral `Backend`,
`Window`, `WebView` and `Tray` interfaces. Everything runs on one OS-locked
UI thread; `Dispatch` is the only door in from other goroutines.

```go
func main() {
	runtime.LockOSThread()

	backend, err := spectacle.New(spectacle.Config{
		ID:      "myapp",           // window class, app-id, icon name
		Name:    "My App",          // notifications, taskbar, toasts
		Icon:    iconPNG,           // PNG bytes, optional
		DataDir: dataDir,           // browsing profiles live here
	})
	if err != nil {
		log.Fatal(err)
	}

	win, _ := backend.NewWindow("My App", spectacle.Rect{X: 100, Y: 100, W: 1280, H: 800})
	view, _ := backend.NewWebView(win, "")   // "" = default profile
	view.Navigate("https://example.com")
	win.OnResize(func(w, h, dpi int) { view.SetBounds(spectacle.Rect{W: w, H: h}) })
	win.Show()

	backend.Run()
}
```

Pages talk to Go through a WebView2-style bridge that works identically on
both platforms: `WebView.PostJSON`/`OnMessage` in Go,
`window.chrome.webview.postMessage`/`addEventListener("message", ...)` in
the page (a polyfill provides the `chrome.webview` API on WebKitGTK).
