//go:build windows

package webview2

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/pietjan/spectacle/internal/w32"
)

// OLE drag-and-drop forwarding. Windowed hosting handled drops inside
// WebView2's own HWNDs; with composition hosting the host window is the
// drop target, so spectacle implements IDropTarget itself and forwards
// to ICoreWebView2CompositionController3 on the view under the cursor.

// IID_IDropTarget {00000122-0000-0000-C000-000000000046}.
var iidDropTarget = windows.GUID{
	Data1: 0x00000122,
	Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46},
}

// IID_IUnknown {00000000-0000-0000-C000-000000000046}.
var iidUnknown = windows.GUID{
	Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46},
}

// IID_ICoreWebView2CompositionController3, from WebView2.h (drag/drop).
var iidCompositionController3 = windows.GUID{
	Data1: 0x9570570e, Data2: 0x4d76, Data3: 0x4361,
	Data4: [8]byte{0x9e, 0xe1, 0xf0, 0x4d, 0x0d, 0xbd, 0xfb, 0x1e},
}

// compositionController3Vtbl walks the full inheritance chain:
// CompositionController(8) + CompositionController2(1) + the drag slots.
type compositionController3Vtbl struct {
	compositionControllerVtbl
	GetAutomationProvider uintptr
	DragEnter             uintptr
	DragLeave             uintptr
	DragOver              uintptr
	Drop                  uintptr
}

type compositionController3 struct {
	vtbl *compositionController3Vtbl
}

// dragTarget returns (querying and caching on first use) the
// controller's drag/drop face, or nil on runtimes without it.
func (c *CompositionController) dragTarget() *compositionController3 {
	if c3, ok := dragFaces[c]; ok {
		return c3
	}
	var c3 *compositionController3
	if p, err := queryInterface(unsafe.Pointer(c), &iidCompositionController3); err == nil {
		c3 = (*compositionController3)(p)
	}
	dragFaces[c] = c3
	return c3
}

// dragFaces caches the QI per controller (UI thread only, like all of
// this package). Entries die with the process; controllers are few.
var dragFaces = map[*CompositionController]*compositionController3{}

// DragEnter forwards an OLE DragEnter. x,y are webview-client
// coordinates; effect is OLE's in/out DWORD, passed straight through.
func (c *CompositionController) DragEnter(data unsafe.Pointer, keyState uint32, x, y int32, effect *uint32) {
	if c3 := c.dragTarget(); c3 != nil {
		pt := uintptr(uint32(x)) | uintptr(uint32(y))<<32
		call(c3.vtbl.DragEnter, unsafe.Pointer(c3), uintptr(data), uintptr(keyState), pt, uintptr(unsafe.Pointer(effect)))
	}
}

// DragOver forwards an OLE DragOver.
func (c *CompositionController) DragOver(keyState uint32, x, y int32, effect *uint32) {
	if c3 := c.dragTarget(); c3 != nil {
		pt := uintptr(uint32(x)) | uintptr(uint32(y))<<32
		call(c3.vtbl.DragOver, unsafe.Pointer(c3), uintptr(keyState), pt, uintptr(unsafe.Pointer(effect)))
	}
}

// DragLeave forwards an OLE DragLeave.
func (c *CompositionController) DragLeave() {
	if c3 := c.dragTarget(); c3 != nil {
		call(c3.vtbl.DragLeave, unsafe.Pointer(c3))
	}
}

// Drop forwards an OLE Drop.
func (c *CompositionController) Drop(data unsafe.Pointer, keyState uint32, x, y int32, effect *uint32) {
	if c3 := c.dragTarget(); c3 != nil {
		pt := uintptr(uint32(x)) | uintptr(uint32(y))<<32
		call(c3.vtbl.Drop, unsafe.Pointer(c3), uintptr(data), uintptr(keyState), pt, uintptr(unsafe.Pointer(effect)))
	}
}

// SupportsDragDrop reports whether the runtime has the drag/drop face.
func (c *CompositionController) SupportsDragDrop() bool {
	return c.dragTarget() != nil
}

// DropRouter is what a window gives RegisterDropTarget: it receives the
// raw OLE drop-target calls with screen coordinates and decides which
// webview they reach. data is the IDataObject*, effect OLE's in/out
// effect word.
type DropRouter interface {
	DragEnter(data unsafe.Pointer, keyState uint32, x, y int32, effect *uint32)
	DragOver(keyState uint32, x, y int32, effect *uint32)
	DragLeave()
	Drop(data unsafe.Pointer, keyState uint32, x, y int32, effect *uint32)
}

// dropTarget is the IDropTarget COM object. One static vtable, per-
// instance router — the comHandler pattern with OLE's four methods.
type dropTarget struct {
	vtbl   *dropTargetVtbl
	router DropRouter
}

type dropTargetVtbl struct {
	iUnknownVtbl
	DragEnter uintptr
	DragOver  uintptr // note: OLE order differs from CompositionController3
	DragLeave uintptr
	Drop      uintptr
}

func splitPoint(pt uintptr) (int32, int32) {
	return int32(uint32(pt)), int32(uint32(pt >> 32))
}

var dropTargetVtable = &dropTargetVtbl{
	iUnknownVtbl: iUnknownVtbl{
		QueryInterface: syscall.NewCallback(func(this unsafe.Pointer, riid *windows.GUID, out *unsafe.Pointer) uintptr {
			if *riid == iidDropTarget || *riid == iidUnknown {
				*out = this
				return 0
			}
			*out = nil
			return 0x80004002 // E_NOINTERFACE
		}),
		AddRef:  syscall.NewCallback(func(this uintptr) uintptr { return 1 }),
		Release: syscall.NewCallback(func(this uintptr) uintptr { return 1 }),
	},
	DragEnter: syscall.NewCallback(func(this, data unsafe.Pointer, keyState, pt uintptr, effect *uint32) uintptr {
		x, y := splitPoint(pt)
		(*dropTarget)(this).router.DragEnter(data, uint32(keyState), x, y, effect)
		return 0
	}),
	DragOver: syscall.NewCallback(func(this unsafe.Pointer, keyState, pt uintptr, effect *uint32) uintptr {
		x, y := splitPoint(pt)
		(*dropTarget)(this).router.DragOver(uint32(keyState), x, y, effect)
		return 0
	}),
	DragLeave: syscall.NewCallback(func(this unsafe.Pointer) uintptr {
		(*dropTarget)(this).router.DragLeave()
		return 0
	}),
	Drop: syscall.NewCallback(func(this, data unsafe.Pointer, keyState, pt uintptr, effect *uint32) uintptr {
		x, y := splitPoint(pt)
		(*dropTarget)(this).router.Drop(data, uint32(keyState), x, y, effect)
		return 0
	}),
}

// pinnedDropTargets keeps drop targets alive while OLE holds them.
var pinnedDropTargets []*dropTarget

// RegisterDropTarget makes hwnd an OLE drop target routed through
// router. Call once per window, after OleInitialize.
func RegisterDropTarget(hwnd uintptr, router DropRouter) error {
	t := &dropTarget{vtbl: dropTargetVtable, router: router}
	pinnedDropTargets = append(pinnedDropTargets, t)
	r, _, _ := w32.RegisterDragDrop.Call(hwnd, uintptr(unsafe.Pointer(t)))
	return checkHR("RegisterDragDrop", r)
}

// AddRefObject/ReleaseObject expose COM refcounting for pointers the
// router holds across calls (the IDataObject during a drag).
func AddRefObject(p unsafe.Pointer)  { addRef(p) }
func ReleaseObject(p unsafe.Pointer) { release(p) }
