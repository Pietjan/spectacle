//go:build windows

package spectacle

import (
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"

	"github.com/wailsapp/go-webview2/webviewloader"

	"github.com/pietjan/spectacle/internal/w32"
	"github.com/pietjan/spectacle/internal/webview2"
)

// backend is the Windows implementation of Backend: one UI
// thread, a Win32 message loop, and WebView2 for the webviews.
type backend struct {
	userDataFolder string
	debug          bool

	main *window

	// env is the shared WebView2 environment (profile-capable runtimes).
	// fallbackEnvs holds one environment per profile for old runtimes
	// without ICoreWebView2Environment10.
	env          *webview2.Environment
	fallbackEnvs map[string]*webview2.Environment

	// Composition (visual) hosting is the default: webviews render into
	// DirectComposition visuals with app-routed input, which is what
	// lets overlays stack and be click-through. windowed flips to the
	// legacy child-HWND path when the runtime can't do composition
	// (pre-Environment10); overlays are unsupported there.
	dcomp    *webview2.CompositionDevice
	windowed bool
	probed   bool

	mu    sync.Mutex
	queue []func()
}

// New prepares the backend. Call from the OS-locked main goroutine.
func New(cfg Config) (Backend, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	conf = cfg
	// Per-monitor-v2 DPI awareness must be set before any window exists.
	w32.SetProcessDpiAwarenessCtx.Call(w32.DpiAwarenessContextPerMonitorAwareV2)
	// Explicit app identity with a registered name and icon, so the
	// shell attributes toasts to the app name and icon rather than
	// the executable name.
	registerIdentity(filepath.Dir(cfg.DataDir))
	if r, _, _ := w32.CoInitializeEx.Call(0, w32.CoinitApartmentThreaded); int32(r) < 0 {
		return nil, fmt.Errorf("win: CoInitializeEx failed: 0x%08x", uint32(r))
	}
	// OLE proper (on top of the STA above) for RegisterDragDrop; without
	// it drag-and-drop silently stays off. S_FALSE (already done) is fine.
	if r, _, _ := w32.OleInitialize.Call(0); int32(r) < 0 {
		log.Printf("win: OleInitialize failed: 0x%08x (drag-and-drop off)", uint32(r))
	}
	if err := ensureRuntime(); err != nil {
		return nil, err
	}
	return &backend{
		userDataFolder: cfg.DataDir,
		debug:          cfg.Debug,
		fallbackEnvs:   map[string]*webview2.Environment{},
	}, nil
}

// ensureRuntime verifies the WebView2 evergreen runtime is installed; if
// not, it tells the user and points them at the bootstrapper.
func ensureRuntime() error {
	if _, err := webviewloader.GetAvailableCoreWebView2BrowserVersionString(""); err == nil {
		return nil
	}
	const bootstrapper = "https://go.microsoft.com/fwlink/p/?LinkId=2124703"
	msg := conf.Name + " needs the Microsoft WebView2 Runtime.\n\nA download page will open; run the installer and start " + conf.Name + " again."
	w32.MessageBox.Call(0,
		uintptr(unsafe.Pointer(utf16Ptr(msg))),
		uintptr(unsafe.Pointer(utf16Ptr(conf.Name))),
		w32.MbIconError|w32.MbOK)
	w32.ShellExecute.Call(0,
		uintptr(unsafe.Pointer(utf16Ptr("open"))),
		uintptr(unsafe.Pointer(utf16Ptr(bootstrapper))), 0, 0, w32.SwShowNormal)
	return fmt.Errorf("win: WebView2 runtime not installed")
}

// NewWindow creates the main window (hidden until Show).
func (b *backend) NewWindow(title string, bounds Rect) (Window, error) {
	w, err := newWindow(b, title, bounds)
	if err != nil {
		return nil, err
	}
	if b.main == nil {
		b.main = w
	}
	return w, nil
}

// NewWebView creates a webview on w in the given profile.
func (b *backend) NewWebView(pw Window, profile string, options ...WebViewOption) (WebView, error) {
	w, ok := pw.(*window)
	if !ok {
		return nil, fmt.Errorf("win: foreign window %T", pw)
	}
	var cfg webViewConfig
	for _, opt := range options {
		opt(&cfg)
	}
	if b.SupportsOverlay() {
		return b.newCompositionWebView(w, profile, cfg)
	}
	if cfg.overlay {
		// Windowed WebView2 controllers own their input HWNDs: a
		// transparent layer would still swallow every click under it.
		return nil, fmt.Errorf("win: overlay webviews need composition hosting, which this runtime lacks")
	}
	ctrl, err := b.controllerFor(w.hwnd, profile)
	if err != nil {
		return nil, err
	}
	return newWebView(w, ctrl, b.debug)
}

// SupportsOverlay probes (once) whether composition hosting is
// available: an Environment10 runtime plus a DirectComposition device.
func (b *backend) SupportsOverlay() bool {
	if b.probed {
		return !b.windowed
	}
	b.probed = true
	b.windowed = true
	if err := b.ensureEnv(); err != nil {
		return false
	}
	if !b.env.SupportsProfiles() { // Environment10 carries composition too
		return false
	}
	dev, err := webview2.NewCompositionDevice()
	if err != nil {
		return false
	}
	b.dcomp = dev
	b.windowed = false
	return true
}

func (b *backend) ensureEnv() error {
	if b.env != nil {
		return nil
	}
	env, err := webview2.NewEnvironment(b.userDataFolder)
	if err != nil {
		return err
	}
	b.env = env
	return nil
}

// newCompositionWebView creates a visual-hosted webview: a composition
// controller rendering into a fresh visual under the window's root.
func (b *backend) newCompositionWebView(w *window, profile string, cfg webViewConfig) (WebView, error) {
	if err := w.ensureComposition(b.dcomp); err != nil {
		return nil, err
	}
	comp, err := b.env.CreateCompositionController(w.hwnd, profile)
	if err != nil {
		return nil, err
	}
	ctrl, err := comp.Controller()
	if err != nil {
		comp.Release()
		return nil, err
	}
	visual, err := b.dcomp.CreateVisual()
	if err != nil {
		ctrl.Close()
		comp.Release()
		return nil, err
	}
	if err := comp.SetRootVisualTarget(visual); err != nil {
		visual.Release()
		ctrl.Close()
		comp.Release()
		return nil, err
	}
	v, err := newWebView(w, ctrl, b.debug)
	if err != nil {
		visual.Release()
		ctrl.Close()
		comp.Release()
		return nil, err
	}
	v.comp, v.visual, v.overlay = comp, visual, cfg.overlay
	if cfg.overlay {
		if err := ctrl.SetTransparentBackground(); err != nil {
			log.Printf("win: %v", err)
		}
	}
	if err := comp.OnCursorChanged(func() {
		v.cursor = comp.Cursor()
		if w.mouseIn == v {
			w32.SetCursor.Call(v.cursor)
		}
	}); err != nil {
		log.Printf("win: %v", err)
	}
	w.attachView(v)
	if err := b.dcomp.Commit(); err != nil {
		log.Printf("win: %v", err)
	}
	return v, nil
}

// controllerFor isolates the windowed-fallback profile strategy: one
// shared environment with named profiles on current runtimes, one
// environment (and user data folder) per profile on runtimes too old
// for Environment10.
func (b *backend) controllerFor(hwnd uintptr, profile string) (*webview2.Controller, error) {
	if err := b.ensureEnv(); err != nil {
		return nil, err
	}
	if profile == "" {
		return b.env.CreateController(hwnd)
	}
	if b.env.SupportsProfiles() {
		return b.env.CreateControllerWithProfile(hwnd, profile)
	}
	env, ok := b.fallbackEnvs[profile]
	if !ok {
		var err error
		env, err = webview2.NewEnvironment(b.userDataFolder + `\profile-` + profile)
		if err != nil {
			return nil, err
		}
		b.fallbackEnvs[profile] = env
	}
	return env.CreateController(hwnd)
}

// Run pumps the message loop until Quit.
func (b *backend) Run() error {
	var msg w32.Msg
	for {
		r, _, _ := w32.GetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		switch int32(r) {
		case -1:
			return fmt.Errorf("win: GetMessage failed")
		case 0:
			return nil // WM_QUIT
		}
		w32.TranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		w32.DispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

// Dispatch schedules f on the UI thread. Safe from any goroutine once
// the main window exists.
func (b *backend) Dispatch(f func()) {
	b.mu.Lock()
	b.queue = append(b.queue, f)
	b.mu.Unlock()
	if b.main != nil {
		w32.PostMessage.Call(b.main.hwnd, wmAppDispatch, 0, 0)
	}
}

func (b *backend) drainDispatch() {
	for {
		b.mu.Lock()
		if len(b.queue) == 0 {
			b.mu.Unlock()
			return
		}
		f := b.queue[0]
		b.queue = b.queue[1:]
		b.mu.Unlock()
		f()
	}
}

// OpenURL opens url in the user's default browser.
func (b *backend) OpenURL(url string) {
	w32.ShellExecute.Call(0,
		uintptr(unsafe.Pointer(utf16Ptr("open"))),
		uintptr(unsafe.Pointer(utf16Ptr(url))), 0, 0, w32.SwShowNormal)
}

// Quit removes the tray icon and ends the message loop.
func (b *backend) Quit() {
	if b.main != nil && b.main.tray != nil {
		b.main.tray.remove()
	}
	w32.PostQuitMessage.Call(0)
}

func utf16FromString(s string) ([]uint16, error) {
	return syscall.UTF16FromString(s)
}
