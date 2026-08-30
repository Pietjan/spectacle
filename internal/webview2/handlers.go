//go:build windows

package webview2

import (
	"sync"
	"syscall"
	"unsafe"
)

// Every WebView2 handler we implement — completed handlers and event
// handlers alike — has the shape IUnknown + Invoke(this, a, b), so one
// generic COM object covers them all. The vtable's four trampolines are
// created once (syscall.NewCallback slots are finite and never freed);
// each instance carries its own Go closure.
// The trampolines type pointer-carrying parameters as unsafe.Pointer so
// no uintptr round trip ever occurs in Go code; parameters that are
// sometimes plain integers (HRESULTs, enum values) arrive the same way
// and are read back with uintptr(p), which is always legal.
type comHandler struct {
	vtbl   *comHandlerVtbl
	invoke func(a, b unsafe.Pointer) uintptr
}

type comHandlerVtbl struct {
	iUnknownVtbl
	Invoke uintptr
}

var handlerVtbl = &comHandlerVtbl{
	iUnknownVtbl: iUnknownVtbl{
		QueryInterface: syscall.NewCallback(func(this unsafe.Pointer, riid uintptr, out *unsafe.Pointer) uintptr {
			// WebView2 only ever asks for interfaces we were handed in
			// as; answering all queries with ourselves is the pattern
			// the reference bindings use too.
			*out = this
			return 0
		}),
		AddRef: syscall.NewCallback(func(this uintptr) uintptr {
			return 1
		}),
		Release: syscall.NewCallback(func(this uintptr) uintptr {
			// Lifetime is owned on the Go side (pinned registry), not by
			// COM refcounting.
			return 1
		}),
	},
	Invoke: syscall.NewCallback(func(this, a, b unsafe.Pointer) uintptr {
		h := (*comHandler)(this)
		return h.invoke(a, b)
	}),
}

// pinned keeps handler objects reachable while COM holds their pointer.
var pinned = struct {
	sync.Mutex
	m map[*comHandler]struct{}
}{m: map[*comHandler]struct{}{}}

// newHandler creates a pinned COM handler around invoke.
func newHandler(invoke func(a, b unsafe.Pointer) uintptr) *comHandler {
	h := &comHandler{vtbl: handlerVtbl, invoke: invoke}
	pinned.Lock()
	pinned.m[h] = struct{}{}
	pinned.Unlock()
	return h
}

// unpin releases a handler for garbage collection. Only call once the COM
// side can no longer invoke it (after removing the event registration or
// closing the controller that held it).
func unpin(h *comHandler) {
	if h == nil {
		return
	}
	pinned.Lock()
	delete(pinned.m, h)
	pinned.Unlock()
}

func (h *comHandler) ptr() uintptr { return uintptr(unsafe.Pointer(h)) }
