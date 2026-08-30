//go:build linux

package native

import (
	"sync"

	"github.com/ebitengine/purego"
)

// GObject signal handlers all have the shape ret f(instance, args...,
// user_data), so a handful of shared trampolines cover every signal
// spectacle connects, dispatching through a registry keyed by the
// user_data value. purego.NewCallback slots are finite and never freed
// (like syscall.NewCallback on Windows) — the five callbacks below are
// the ONLY NewCallback calls allowed in this codebase's Linux port.
//
// Registry entries are freed by GLib itself: the GDestroyNotify passed
// to g_signal_connect_data (and g_idle_add_full) fires when the handler
// is disconnected or the source removed, mirroring win/webview2's
// unpin discipline.

type conn struct {
	fn func(args []uintptr) uintptr
}

var registry = struct {
	sync.Mutex
	m    map[uintptr]*conn
	next uintptr
}{m: map[uintptr]*conn{}, next: 1}

func register(fn func(args []uintptr) uintptr) uintptr {
	registry.Lock()
	defer registry.Unlock()
	key := registry.next
	registry.next++
	registry.m[key] = &conn{fn: fn}
	return key
}

func lookup(key uintptr) *conn {
	registry.Lock()
	defer registry.Unlock()
	return registry.m[key]
}

var (
	tramp0 = purego.NewCallback(func(inst, data uintptr) uintptr {
		if c := lookup(data); c != nil {
			return c.fn([]uintptr{inst})
		}
		return 0
	})
	tramp1 = purego.NewCallback(func(inst, a, data uintptr) uintptr {
		if c := lookup(data); c != nil {
			return c.fn([]uintptr{inst, a})
		}
		return 0
	})
	tramp2 = purego.NewCallback(func(inst, a, b, data uintptr) uintptr {
		if c := lookup(data); c != nil {
			return c.fn([]uintptr{inst, a, b})
		}
		return 0
	})
	// GSourceFunc for idle/timeout sources; always one-shot.
	sourceTramp = purego.NewCallback(func(data uintptr) uintptr {
		if c := lookup(data); c != nil {
			c.fn(nil)
		}
		return 0 // G_SOURCE_REMOVE
	})
	destroyTramp = purego.NewCallback(func(data uintptr) uintptr {
		registry.Lock()
		delete(registry.m, data)
		registry.Unlock()
		return 0
	})
)

// Connect wires a GObject signal to fn. extraArgs is the number of
// signal arguments between the instance and user_data (0, 1, or 2).
// fn receives [instance, args...]; its return value is the handler's.
// The registry entry lives until the instance is finalized.
func Connect(instance uintptr, signal string, extraArgs int, fn func(args []uintptr) uintptr) {
	tramp := tramp0
	switch extraArgs {
	case 1:
		tramp = tramp1
	case 2:
		tramp = tramp2
	}
	GSignalConnectData(instance, signal, tramp, register(fn), destroyTramp, 0)
}

// ScheduleIdle runs fn once on the GLib main loop at the given priority.
// Thread-safe: g_idle_add_full wakes the main context itself. This is
// the backend's Dispatch primitive.
func ScheduleIdle(priority int32, fn func()) {
	key := register(func([]uintptr) uintptr { fn(); return 0 })
	GIdleAddFull(priority, sourceTramp, key, destroyTramp)
}

// ScheduleTimeout runs fn once after ms milliseconds on the main loop.
func ScheduleTimeout(ms uint32, fn func()) {
	key := register(func([]uintptr) uintptr { fn(); return 0 })
	GTimeoutAddFull(PriorityDefault, ms, sourceTramp, key, destroyTramp)
}
