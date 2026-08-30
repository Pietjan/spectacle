//go:build windows

package webview2

import (
	"fmt"
	"unsafe"

	"github.com/wailsapp/go-webview2/webviewloader"
	"golang.org/x/sys/windows"
)

// IID_ICoreWebView2Environment10, from WebView2.h.
var iidEnvironment10 = windows.GUID{
	Data1: 0xee0eb9df, Data2: 0x6f12, Data3: 0x46ce,
	Data4: [8]byte{0xb5, 0x3f, 0x3f, 0x47, 0xb9, 0xc9, 0x28, 0xe0},
}

// Environment wraps ICoreWebView2Environment (one browser runtime
// instance; all webviews and profiles hang off it).
type Environment struct {
	vtbl *environmentVtbl
}

type environmentVtbl struct {
	iUnknownVtbl
	CreateCoreWebView2Controller     uintptr
	CreateWebResourceResponse        uintptr
	GetBrowserVersionString          uintptr
	AddNewBrowserVersionAvailable    uintptr
	RemoveNewBrowserVersionAvailable uintptr
}

// environment10 wraps ICoreWebView2Environment10. The vtable prefix walks
// the whole Environment..Environment9 inheritance chain.
type environment10 struct {
	vtbl *environment10Vtbl
}

type environment10Vtbl struct {
	environmentVtbl
	// Environment2..9.
	CreateWebResourceRequest                uintptr
	CreateCoreWebView2CompositionController uintptr
	CreateCoreWebView2PointerInfo           uintptr
	GetAutomationProviderForWindow          uintptr
	AddBrowserProcessExited                 uintptr
	RemoveBrowserProcessExited              uintptr
	CreatePrintSettings                     uintptr
	GetUserDataFolder                       uintptr
	AddProcessInfosChanged                  uintptr
	RemoveProcessInfosChanged               uintptr
	GetProcessInfos                         uintptr
	CreateContextMenuItem                   uintptr
	// Environment10.
	CreateCoreWebView2ControllerOptions     uintptr
	CreateCoreWebView2ControllerWithOptions uintptr
	CreateCompositionControllerWithOptions  uintptr
}

type controllerOptions struct {
	vtbl *controllerOptionsVtbl
}

type controllerOptionsVtbl struct {
	iUnknownVtbl
	GetProfileName            uintptr
	PutProfileName            uintptr
	GetIsInPrivateModeEnabled uintptr
	PutIsInPrivateModeEnabled uintptr
}

// environmentCompleted adapts the loader's callback interface.
type environmentCompleted struct {
	env  *Environment
	hr   int32
	done bool
}

func (h *environmentCompleted) EnvironmentCompleted(errorCode webviewloader.HRESULT, created *webviewloader.ICoreWebView2Environment) webviewloader.HRESULT {
	h.hr = int32(errorCode)
	if errorCode >= 0 && created != nil {
		// The pointer is borrowed for the duration of the callback;
		// AddRef to keep the environment alive.
		created.AddRef()
		h.env = (*Environment)(unsafe.Pointer(created))
	}
	h.done = true
	return 0
}

// NewEnvironment creates the WebView2 environment for userDataFolder,
// pumping messages until creation completes. UI thread only.
func NewEnvironment(userDataFolder string) (*Environment, error) {
	h := &environmentCompleted{}
	err := webviewloader.CreateCoreWebView2EnvironmentWithOptions(h,
		webviewloader.WithUserDataFolder(userDataFolder))
	if err != nil {
		return nil, fmt.Errorf("webview2: create environment: %w", err)
	}
	if err := awaitFlag(&h.done); err != nil {
		return nil, err
	}
	if h.hr < 0 {
		return nil, fmt.Errorf("webview2: create environment: HRESULT 0x%08x", uint32(h.hr))
	}
	return h.env, nil
}

// awaitController runs a controller-producing COM call and pumps until its
// completed handler fires. create is passed the handler to hand to COM.
func awaitController(op string, create func(handler *comHandler) uintptr) (*Controller, error) {
	var (
		done bool
		hr   int32
		ctrl *Controller
	)
	h := newHandler(func(errCode, controller unsafe.Pointer) uintptr {
		hr = int32(uintptr(errCode))
		if hr >= 0 && controller != nil {
			addRef(controller)
			ctrl = (*Controller)(controller)
		}
		done = true
		return 0
	})
	defer unpin(h) // completed handlers fire once; safe to unpin after

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

// CreateController creates a controller in the default profile.
func (e *Environment) CreateController(hwnd uintptr) (*Controller, error) {
	return awaitController("CreateCoreWebView2Controller", func(h *comHandler) uintptr {
		return call(e.vtbl.CreateCoreWebView2Controller, unsafe.Pointer(e), hwnd, h.ptr())
	})
}

// SupportsProfiles reports whether the installed runtime exposes
// ICoreWebView2Environment10 (multi-profile support).
func (e *Environment) SupportsProfiles() bool {
	p, err := queryInterface(unsafe.Pointer(e), &iidEnvironment10)
	if err != nil {
		return false
	}
	release(p)
	return true
}

// CreateControllerWithProfile creates a controller bound to the named
// profile under the environment's user data folder.
func (e *Environment) CreateControllerWithProfile(hwnd uintptr, profile string) (*Controller, error) {
	p, err := queryInterface(unsafe.Pointer(e), &iidEnvironment10)
	if err != nil {
		return nil, fmt.Errorf("webview2: runtime has no profile support: %w", err)
	}
	env10 := (*environment10)(p)
	defer release(p)

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

	return awaitController("CreateCoreWebView2ControllerWithOptions", func(h *comHandler) uintptr {
		return call(env10.vtbl.CreateCoreWebView2ControllerWithOptions, p, hwnd, uintptr(optsPtr), h.ptr())
	})
}
