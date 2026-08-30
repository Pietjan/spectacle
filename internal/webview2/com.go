//go:build windows

// Package webview2 is a pure-Go binding to the WebView2 COM
// API: hand-written vtables for exactly the interfaces the app needs, a
// generic callback object for the completed/event handlers, and thin Go
// wrappers. Layouts and IIDs are transcribed from WebView2.h.
//
// Everything here must run on the single OS-locked UI thread.
package webview2

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/pietjan/spectacle/internal/w32"
)

// call invokes a COM method through its vtable slot.
func call(method uintptr, this unsafe.Pointer, args ...uintptr) uintptr {
	full := append([]uintptr{uintptr(this)}, args...)
	r, _, _ := syscall.SyscallN(method, full...)
	return r
}

// checkHR turns a failed HRESULT into an error.
func checkHR(op string, r uintptr) error {
	// HRESULT failure bit is the sign bit of the low 32 bits.
	if int32(r) < 0 {
		return fmt.Errorf("webview2: %s: %w", op, syscall.Errno(r))
	}
	return nil
}

func utf16Ptr(s string) *uint16 {
	p, err := windows.UTF16PtrFromString(s)
	if err != nil {
		// Only fails on interior NULs, which never come from our inputs.
		panic(err)
	}
	return p
}

// takeCoTaskString copies a CoTaskMem LPWSTR into a Go string and frees it.
func takeCoTaskString(p *uint16) string {
	if p == nil {
		return ""
	}
	s := windows.UTF16PtrToString(p)
	w32.CoTaskMemFree.Call(uintptr(unsafe.Pointer(p)))
	return s
}

// iUnknown is the head of every COM interface.
type iUnknown struct {
	vtbl *iUnknownVtbl
}

type iUnknownVtbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
}

func addRef(p unsafe.Pointer) {
	u := (*iUnknown)(p)
	call(u.vtbl.AddRef, p)
}

func release(p unsafe.Pointer) {
	if p == nil {
		return
	}
	u := (*iUnknown)(p)
	call(u.vtbl.Release, p)
}

// queryInterface asks p for the interface identified by iid.
func queryInterface(p unsafe.Pointer, iid *windows.GUID) (unsafe.Pointer, error) {
	u := (*iUnknown)(p)
	var out unsafe.Pointer
	r := call(u.vtbl.QueryInterface, p,
		uintptr(unsafe.Pointer(iid)),
		uintptr(unsafe.Pointer(&out)))
	if err := checkHR("QueryInterface", r); err != nil {
		return nil, err
	}
	return out, nil
}

// awaitFlag pumps the message loop until *done. WebView2's completed
// handlers are delivered through the Windows message queue, so creation
// APIs must pump to make progress. Startup-only; never nest during Run.
func awaitFlag(done *bool) error {
	var msg w32.Msg
	for !*done {
		r, _, _ := w32.GetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		switch int32(r) {
		case -1:
			return fmt.Errorf("webview2: GetMessage failed")
		case 0:
			return fmt.Errorf("webview2: quit while waiting for WebView2")
		}
		w32.TranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		w32.DispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}
	return nil
}
