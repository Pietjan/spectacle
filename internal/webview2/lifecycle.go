//go:build windows

package webview2

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// IID_ICoreWebView2_19, from WebView2.h (memory usage target API).
var iidCoreWebView2_19 = windows.GUID{
	Data1: 0x6921f954, Data2: 0x79b0, Data3: 0x437f,
	Data4: [8]byte{0xa9, 0x97, 0xc8, 0x58, 0x11, 0x89, 0x7c, 0x68},
}

type coreWebView2_19 struct {
	vtbl *coreWebView2_19Vtbl
}

type coreWebView2_19Vtbl struct {
	coreWebView2_15Vtbl
	// ICoreWebView2_16.
	Print            uintptr
	ShowPrintUI      uintptr
	PrintToPdfStream uintptr
	// ICoreWebView2_17.
	PostSharedBufferToScript uintptr
	// ICoreWebView2_18.
	AddLaunchingExternalUriScheme    uintptr
	RemoveLaunchingExternalUriScheme uintptr
	// ICoreWebView2_19.
	GetMemoryUsageTargetLevel uintptr
	PutMemoryUsageTargetLevel uintptr
}

// COREWEBVIEW2_MEMORY_USAGE_TARGET_LEVEL values.
const (
	memoryTargetNormal = 0
	memoryTargetLow    = 1
)

// SetMemoryTargetLow trims (low=true) or restores (low=false) the
// webview's memory footprint. Unlike suspension, script keeps running —
// the right setting for hidden-but-listening service views. No-op error
// on runtimes without the API.
func (w *CoreWebView2) SetMemoryTargetLow(low bool) error {
	p, err := queryInterface(unsafe.Pointer(w), &iidCoreWebView2_19)
	if err != nil {
		return err
	}
	defer release(p)
	v19 := (*coreWebView2_19)(p)
	level := uintptr(memoryTargetNormal)
	if low {
		level = memoryTargetLow
	}
	return checkHR("put_MemoryUsageTargetLevel", call(v19.vtbl.PutMemoryUsageTargetLevel, p, level))
}

// TrySuspend asks the browser to suspend the webview (script stops; use
// only for views with no background duties). The webview must already be
// hidden. Best-effort: the completion result is ignored.
func (w *CoreWebView2) TrySuspend() error {
	p, err := queryInterface(unsafe.Pointer(w), &iidCoreWebView2_15)
	if err != nil {
		return err
	}
	defer release(p)
	v15 := (*coreWebView2_15)(p)
	var h *comHandler
	h = newHandler(func(errCode, ok unsafe.Pointer) uintptr {
		unpin(h)
		return 0
	})
	hr := call(v15.vtbl.TrySuspend, p, h.ptr())
	if int32(hr) < 0 {
		unpin(h)
	}
	return checkHR("TrySuspend", hr)
}

// Resume undoes TrySuspend. Safe to call on a non-suspended webview.
func (w *CoreWebView2) Resume() error {
	p, err := queryInterface(unsafe.Pointer(w), &iidCoreWebView2_15)
	if err != nil {
		return err
	}
	defer release(p)
	v15 := (*coreWebView2_15)(p)
	return checkHR("Resume", call(v15.vtbl.Resume, p))
}
