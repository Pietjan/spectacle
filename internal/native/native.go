//go:build linux

// Package native is a hand-rolled binding to GTK4, GLib/GObject,
// and WebKitGTK 6.0 — the Linux analogue of the win/w32 + win/webview2
// layers. Libraries are dlopen'd at runtime via purego, so builds need
// no C toolchain or headers and binaries start on any distro that has
// the runtime libraries installed.
//
// Everything here must run on the single OS-locked UI thread, except
// where noted (ScheduleIdle is thread-safe).
package native

import (
	"fmt"

	"github.com/ebitengine/purego"
)

// libraries in dependency order, with the Ubuntu/Debian package that
// provides each (for the error message).
var libraries = []struct {
	name string
	pkg  string
	regs []registration
}{
	{"libglib-2.0.so.0", "libglib2.0-0", glibFuncs},
	{"libgobject-2.0.so.0", "libglib2.0-0", gobjectFuncs},
	{"libgio-2.0.so.0", "libglib2.0-0", gioFuncs},
	{"libgtk-4.so.1", "libgtk-4-1", gtkFuncs},
	{"libjavascriptcoregtk-6.0.so.1", "libjavascriptcoregtk-6.0-1", jscFuncs},
	{"libwebkitgtk-6.0.so.4", "libwebkitgtk-6.0-4", webkitFuncs},
}

type registration struct {
	fptr any
	name string
}

var loaded bool

// Load opens the toolkit libraries and resolves every function spectacle
// uses. Symbols are pre-checked so a missing library or too-old version
// yields a named error instead of a panic.
func Load() error {
	if loaded {
		return nil
	}
	for _, lib := range libraries {
		handle, err := purego.Dlopen(lib.name, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if err != nil {
			return fmt.Errorf("native: %s not found (install %s): %w", lib.name, lib.pkg, err)
		}
		for _, r := range lib.regs {
			if _, err := purego.Dlsym(handle, r.name); err != nil {
				return fmt.Errorf("native: %s lacks %s — %s too old?: %w", lib.name, r.name, lib.pkg, err)
			}
			purego.RegisterLibFunc(r.fptr, handle, r.name)
		}
	}
	loaded = true
	return nil
}
