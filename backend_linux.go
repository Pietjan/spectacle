//go:build linux

// The GTK4 + WebKitGTK 6.0 backend, bound at runtime via purego (no
// cgo). One OS-locked UI thread runs the GLib main loop; Dispatch is
// the only door in from other goroutines.
package spectacle

import (
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/pietjan/spectacle/internal/native"
)

// backend is the Linux implementation of Backend.
type backend struct {
	dataDir string
	debug   bool

	main     *window
	sessions *sessionManager
	loop     uintptr // GMainLoop*

	mu    sync.Mutex
	queue []func()
}

// New prepares the backend. Call from the OS-locked main goroutine.
func New(cfg Config) (Backend, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	conf = cfg
	// WebKitGTK's dmabuf renderer produces blank views under WSLg;
	// disable it there (overridable) so the dev loop works.
	if _, err := os.Stat("/mnt/wslg"); err == nil {
		if os.Getenv("WEBKIT_DISABLE_DMABUF_RENDERER") == "" {
			os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")
		}
	}
	if err := native.Load(); err != nil {
		return nil, err
	}
	// The program name becomes the Wayland app-id, which shells match
	// against the desktop entry registerIdentity installs.
	native.GSetPrgname(cfg.ID)
	registerIdentity()
	native.GtkInit()
	applyTheme()
	return &backend{
		dataDir:  cfg.DataDir,
		debug:    cfg.Debug,
		sessions: newSessionManager(cfg.DataDir),
		loop:     native.GMainLoopNew(0, 0),
	}, nil
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
func (b *backend) NewWebView(pw Window, profile string) (WebView, error) {
	w, ok := pw.(*window)
	if !ok {
		return nil, fmt.Errorf("linux: foreign window %T", pw)
	}
	return newWebView(b, w, profile)
}

// Run pumps the GLib main loop until Quit.
func (b *backend) Run() error {
	native.GMainLoopRun(b.loop)
	return nil
}

// Dispatch schedules f on the UI thread. Safe from any goroutine.
func (b *backend) Dispatch(f func()) {
	b.mu.Lock()
	b.queue = append(b.queue, f)
	b.mu.Unlock()
	native.ScheduleIdle(native.PriorityDefault, b.drainDispatch)
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
	_ = exec.Command("xdg-open", url).Start()
}

// Quit tears down the tray and ends the main loop.
func (b *backend) Quit() {
	if b.main != nil && b.main.tray != nil {
		b.main.tray.remove()
	}
	native.GMainLoopQuit(b.loop)
}
