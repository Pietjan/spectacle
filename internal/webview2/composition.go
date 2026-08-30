//go:build windows

package webview2

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/pietjan/spectacle/internal/w32"
)

// Composition (visual) hosting: webviews render into DirectComposition
// visuals instead of child HWNDs, so views can stack, be transparent,
// and have their input routed by the app. Vtables are transcribed from
// dcomp.h — beware two traps baked into the layouts below: MSVC places
// overloaded virtuals in REVERSE declaration order (the animation
// variants come before the float ones), and float-argument slots can't
// be called through syscall at all (floats ride XMM registers), which
// is why positioning goes through SetTransform's matrix POINTER.

// IID_IDCompositionDesktopDevice, from dcomp.h.
var iidDCompDesktopDevice = windows.GUID{
	Data1: 0x5f4633fe, Data2: 0x1e08, Data3: 0x4cb8,
	Data4: [8]byte{0x8c, 0x75, 0xce, 0x24, 0x33, 0x3f, 0x56, 0x02},
}

// IID_ICoreWebView2Controller, from WebView2.h.
var iidController = windows.GUID{
	Data1: 0x4d00c0d1, Data2: 0x9434, Data3: 0x4eb6,
	Data4: [8]byte{0x80, 0x78, 0x86, 0x97, 0xa5, 0x60, 0x33, 0x4f},
}

// dcompDevice2Vtbl is IDCompositionDevice2 (IDCompositionDesktopDevice
// extends it with the three methods appended in desktopDeviceVtbl).
type dcompDevice2Vtbl struct {
	iUnknownVtbl
	Commit                     uintptr
	WaitForCommitCompletion    uintptr
	GetFrameStatistics         uintptr
	CreateVisual               uintptr
	CreateSurfaceFactory       uintptr
	CreateSurface              uintptr
	CreateVirtualSurface       uintptr
	CreateTranslateTransform   uintptr
	CreateScaleTransform       uintptr
	CreateRotateTransform      uintptr
	CreateSkewTransform        uintptr
	CreateMatrixTransform      uintptr
	CreateTransformGroup       uintptr
	CreateTranslateTransform3D uintptr
	CreateScaleTransform3D     uintptr
	CreateRotateTransform3D    uintptr
	CreateMatrixTransform3D    uintptr
	CreateTransform3DGroup     uintptr
	CreateEffectGroup          uintptr
	CreateRectangleClip        uintptr
	CreateAnimation            uintptr
}

type desktopDeviceVtbl struct {
	dcompDevice2Vtbl
	CreateTargetForHwnd     uintptr
	CreateSurfaceFromHandle uintptr
	CreateSurfaceFromHwnd   uintptr
}

// dcompVisualVtbl is IDCompositionVisual (CreateVisual actually returns
// an IDCompositionVisual2, whose extra methods sit past these).
// Overload pairs are reversed: animation slot first, value slot second.
type dcompVisualVtbl struct {
	iUnknownVtbl
	SetOffsetXAnim             uintptr
	SetOffsetX                 uintptr // float arg — NOT callable via syscall
	SetOffsetYAnim             uintptr
	SetOffsetY                 uintptr // float arg — NOT callable via syscall
	SetTransformObject         uintptr
	SetTransform               uintptr // const D2D_MATRIX_3X2_F& — a pointer
	SetTransformParent         uintptr
	SetEffect                  uintptr
	SetBitmapInterpolationMode uintptr
	SetBorderMode              uintptr
	SetClipObject              uintptr
	SetClip                    uintptr
	SetContent                 uintptr
	AddVisual                  uintptr
	RemoveVisual               uintptr
	RemoveAllVisuals           uintptr
	SetCompositeMode           uintptr
}

type dcompTargetVtbl struct {
	iUnknownVtbl
	SetRoot uintptr
}

// CompositionDevice wraps IDCompositionDesktopDevice.
type CompositionDevice struct {
	vtbl *desktopDeviceVtbl
}

// CompositionTarget wraps IDCompositionTarget: binds a visual tree to
// an HWND.
type CompositionTarget struct {
	vtbl *dcompTargetVtbl
}

// Visual wraps IDCompositionVisual2.
type Visual struct {
	vtbl *dcompVisualVtbl
}

// NewCompositionDevice creates a DirectComposition device with no
// rendering device — visuals only, which is all hosting WebView2 needs.
func NewCompositionDevice() (*CompositionDevice, error) {
	var out unsafe.Pointer
	r, _, _ := w32.DCompositionCreateDevice2.Call(0,
		uintptr(unsafe.Pointer(&iidDCompDesktopDevice)),
		uintptr(unsafe.Pointer(&out)))
	if err := checkHR("DCompositionCreateDevice2", r); err != nil {
		return nil, err
	}
	return (*CompositionDevice)(out), nil
}

// CreateTargetForHwnd binds hwnd's client area to a new composition
// target (topmost over the window's own painting).
func (d *CompositionDevice) CreateTargetForHwnd(hwnd uintptr) (*CompositionTarget, error) {
	var out unsafe.Pointer
	r := call(d.vtbl.CreateTargetForHwnd, unsafe.Pointer(d), hwnd, 1, uintptr(unsafe.Pointer(&out)))
	if err := checkHR("CreateTargetForHwnd", r); err != nil {
		return nil, err
	}
	return (*CompositionTarget)(out), nil
}

// CreateVisual creates an empty visual.
func (d *CompositionDevice) CreateVisual() (*Visual, error) {
	var out unsafe.Pointer
	r := call(d.vtbl.CreateVisual, unsafe.Pointer(d), uintptr(unsafe.Pointer(&out)))
	if err := checkHR("CreateVisual", r); err != nil {
		return nil, err
	}
	return (*Visual)(out), nil
}

// Commit publishes all pending visual-tree changes to the compositor.
func (d *CompositionDevice) Commit() error {
	return checkHR("dcomp Commit", call(d.vtbl.Commit, unsafe.Pointer(d)))
}

// SetRoot makes v the target's root visual.
func (t *CompositionTarget) SetRoot(v *Visual) error {
	return checkHR("SetRoot", call(t.vtbl.SetRoot, unsafe.Pointer(t), uintptr(unsafe.Pointer(v))))
}

// SetTranslation positions the visual at (x, y) within its parent via a
// translation matrix (the offset setters take floats, which syscall
// cannot pass).
func (v *Visual) SetTranslation(x, y int32) error {
	m := [6]float32{1, 0, 0, 1, float32(x), float32(y)}
	return checkHR("SetTransform", call(v.vtbl.SetTransform, unsafe.Pointer(v), uintptr(unsafe.Pointer(&m))))
}

// AddVisual appends child as the topmost child of v.
func (v *Visual) AddVisual(child *Visual) error {
	// With a nil reference, insertAbove=FALSE appends to the END of the
	// child list, and children render in list order — last on top.
	// (TRUE prepends, which paints the new child underneath.)
	return checkHR("AddVisual", call(v.vtbl.AddVisual, unsafe.Pointer(v),
		uintptr(unsafe.Pointer(child)), 0, 0))
}

// RemoveVisual detaches child from v.
func (v *Visual) RemoveVisual(child *Visual) error {
	return checkHR("RemoveVisual", call(v.vtbl.RemoveVisual, unsafe.Pointer(v), uintptr(unsafe.Pointer(child))))
}

// Release drops the visual's COM reference.
func (v *Visual) Release() { release(unsafe.Pointer(v)) }

// compositionControllerVtbl is ICoreWebView2CompositionController.
type compositionControllerVtbl struct {
	iUnknownVtbl
	GetRootVisualTarget uintptr
	PutRootVisualTarget uintptr
	SendMouseInput      uintptr
	SendPointerInput    uintptr
	GetCursor           uintptr
	GetSystemCursorId   uintptr
	AddCursorChanged    uintptr
	RemoveCursorChanged uintptr
}

// CompositionController wraps ICoreWebView2CompositionController — the
// visual-hosting face of a controller. Controller() returns the plain
// face for bounds/visibility/focus.
type CompositionController struct {
	vtbl *compositionControllerVtbl
}

// Controller returns the ICoreWebView2Controller face of this
// composition controller.
func (c *CompositionController) Controller() (*Controller, error) {
	p, err := queryInterface(unsafe.Pointer(c), &iidController)
	if err != nil {
		return nil, err
	}
	return (*Controller)(p), nil
}

// SetRootVisualTarget hands WebView2 the visual it should render into.
func (c *CompositionController) SetRootVisualTarget(v *Visual) error {
	return checkHR("put_RootVisualTarget", call(c.vtbl.PutRootVisualTarget,
		unsafe.Pointer(c), uintptr(unsafe.Pointer(v))))
}

// SendMouseInput forwards one mouse event. kind is the WM_* message
// code, keys the wParam modifier mask, data the wheel delta or xbutton,
// and x,y the position in the webview's own client coordinates.
// (POINT is 8 bytes, passed by value in one register on Win64.)
func (c *CompositionController) SendMouseInput(kind uint32, keys uint32, data uint32, x, y int32) error {
	pt := uintptr(uint32(x)) | uintptr(uint32(y))<<32
	return checkHR("SendMouseInput", call(c.vtbl.SendMouseInput, unsafe.Pointer(c),
		uintptr(kind), uintptr(keys), uintptr(data), pt))
}

// Cursor returns the HCURSOR the webview wants shown right now.
func (c *CompositionController) Cursor() uintptr {
	var cur uintptr
	call(c.vtbl.GetCursor, unsafe.Pointer(c), uintptr(unsafe.Pointer(&cur)))
	return cur
}

// OnCursorChanged fires fn whenever the webview's desired cursor
// changes. The registration lives for the controller's lifetime.
func (c *CompositionController) OnCursorChanged(fn func()) error {
	h := newHandler(func(a, b unsafe.Pointer) uintptr {
		fn()
		return 0
	})
	var token uint64
	r := call(c.vtbl.AddCursorChanged, unsafe.Pointer(c), h.ptr(), uintptr(unsafe.Pointer(&token)))
	if err := checkHR("add_CursorChanged", r); err != nil {
		unpin(h)
		return err
	}
	return nil
}

// Release drops the composition controller's COM reference (the plain
// Controller face holds its own).
func (c *CompositionController) Release() { release(unsafe.Pointer(c)) }

// awaitCompositionController mirrors awaitController for the
// composition-controller completed handler.
func awaitCompositionController(op string, create func(handler *comHandler) uintptr) (*CompositionController, error) {
	var (
		done bool
		hr   int32
		ctrl *CompositionController
	)
	h := newHandler(func(errCode, controller unsafe.Pointer) uintptr {
		hr = int32(uintptr(errCode))
		if hr >= 0 && controller != nil {
			addRef(controller)
			ctrl = (*CompositionController)(controller)
		}
		done = true
		return 0
	})
	defer unpin(h)

	if r := create(h); int32(r) < 0 {
		return nil, checkHR(op, r)
	}
	if err := awaitFlag(&done); err != nil {
		return nil, err
	}
	if hr < 0 {
		return nil, fmt.Errorf("webview2: %s: HRESULT 0x%08x", op, uint32(hr))
	}
	return ctrl, nil
}

// CreateCompositionController creates a composition-hosted controller,
// in the named profile when profile is non-empty. Requires a runtime
// with ICoreWebView2Environment10.
func (e *Environment) CreateCompositionController(hwnd uintptr, profile string) (*CompositionController, error) {
	p, err := queryInterface(unsafe.Pointer(e), &iidEnvironment10)
	if err != nil {
		return nil, fmt.Errorf("webview2: runtime too old for composition hosting: %w", err)
	}
	env10 := (*environment10)(p)
	defer release(p)

	if profile == "" {
		return awaitCompositionController("CreateCoreWebView2CompositionController", func(h *comHandler) uintptr {
			return call(env10.vtbl.CreateCoreWebView2CompositionController, p, hwnd, h.ptr())
		})
	}

	var optsPtr unsafe.Pointer
	r := call(env10.vtbl.CreateCoreWebView2ControllerOptions, p, uintptr(unsafe.Pointer(&optsPtr)))
	if err := checkHR("CreateCoreWebView2ControllerOptions", r); err != nil {
		return nil, err
	}
	defer release(optsPtr)
	opts := (*controllerOptions)(optsPtr)
	r = call(opts.vtbl.PutProfileName, optsPtr, uintptr(unsafe.Pointer(utf16Ptr(profile))))
	if err := checkHR("put_ProfileName", r); err != nil {
		return nil, err
	}
	return awaitCompositionController("CreateCoreWebView2CompositionControllerWithOptions", func(h *comHandler) uintptr {
		return call(env10.vtbl.CreateCompositionControllerWithOptions, p, hwnd, uintptr(optsPtr), h.ptr())
	})
}
