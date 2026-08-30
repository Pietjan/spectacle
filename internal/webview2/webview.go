//go:build windows

package webview2

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// CoreWebView2 wraps ICoreWebView2: navigation, script, messaging, events.
type CoreWebView2 struct {
	vtbl *coreWebView2Vtbl
}

type coreWebView2Vtbl struct {
	iUnknownVtbl
	GetSettings                            uintptr
	GetSource                              uintptr
	Navigate                               uintptr
	NavigateToString                       uintptr
	AddNavigationStarting                  uintptr
	RemoveNavigationStarting               uintptr
	AddContentLoading                      uintptr
	RemoveContentLoading                   uintptr
	AddSourceChanged                       uintptr
	RemoveSourceChanged                    uintptr
	AddHistoryChanged                      uintptr
	RemoveHistoryChanged                   uintptr
	AddNavigationCompleted                 uintptr
	RemoveNavigationCompleted              uintptr
	AddFrameNavigationStarting             uintptr
	RemoveFrameNavigationStarting          uintptr
	AddFrameNavigationCompleted            uintptr
	RemoveFrameNavigationCompleted         uintptr
	AddScriptDialogOpening                 uintptr
	RemoveScriptDialogOpening              uintptr
	AddPermissionRequested                 uintptr
	RemovePermissionRequested              uintptr
	AddProcessFailed                       uintptr
	RemoveProcessFailed                    uintptr
	AddScriptToExecuteOnDocumentCreated    uintptr
	RemoveScriptToExecuteOnDocumentCreated uintptr
	ExecuteScript                          uintptr
	CapturePreview                         uintptr
	Reload                                 uintptr
	PostWebMessageAsJson                   uintptr
	PostWebMessageAsString                 uintptr
	AddWebMessageReceived                  uintptr
	RemoveWebMessageReceived               uintptr
	CallDevToolsProtocolMethod             uintptr
	GetBrowserProcessID                    uintptr
	GetCanGoBack                           uintptr
	GetCanGoForward                        uintptr
	GoBack                                 uintptr
	GoForward                              uintptr
	GetDevToolsProtocolEventReceiver       uintptr
	Stop                                   uintptr
	AddNewWindowRequested                  uintptr
	RemoveNewWindowRequested               uintptr
	AddDocumentTitleChanged                uintptr
	RemoveDocumentTitleChanged             uintptr
	GetDocumentTitle                       uintptr
	AddHostObjectToScript                  uintptr
	RemoveHostObjectFromScript             uintptr
	OpenDevToolsWindow                     uintptr
	AddContainsFullScreenElementChanged    uintptr
	RemoveContainsFullScreenElementChanged uintptr
	GetContainsFullScreenElement           uintptr
	AddWebResourceRequested                uintptr
	RemoveWebResourceRequested             uintptr
	AddWebResourceRequestedFilter          uintptr
	RemoveWebResourceRequestedFilter       uintptr
	AddWindowCloseRequested                uintptr
	RemoveWindowCloseRequested             uintptr
}

type settings struct {
	vtbl *settingsVtbl
}

type settingsVtbl struct {
	iUnknownVtbl
	GetIsScriptEnabled                uintptr
	PutIsScriptEnabled                uintptr
	GetIsWebMessageEnabled            uintptr
	PutIsWebMessageEnabled            uintptr
	GetAreDefaultScriptDialogsEnabled uintptr
	PutAreDefaultScriptDialogsEnabled uintptr
	GetIsStatusBarEnabled             uintptr
	PutIsStatusBarEnabled             uintptr
	GetAreDevToolsEnabled             uintptr
	PutAreDevToolsEnabled             uintptr
	GetAreDefaultContextMenusEnabled  uintptr
	PutAreDefaultContextMenusEnabled  uintptr
	GetAreHostObjectsAllowed          uintptr
	PutAreHostObjectsAllowed          uintptr
	GetIsZoomControlEnabled           uintptr
	PutIsZoomControlEnabled           uintptr
	GetIsBuiltInErrorPageEnabled      uintptr
	PutIsBuiltInErrorPageEnabled      uintptr
}

type webMessageReceivedEventArgs struct {
	vtbl *webMessageReceivedEventArgsVtbl
}

type webMessageReceivedEventArgsVtbl struct {
	iUnknownVtbl
	GetSource                uintptr
	GetWebMessageAsJson      uintptr
	TryGetWebMessageAsString uintptr
}

type navigationCompletedEventArgs struct {
	vtbl *navigationCompletedEventArgsVtbl
}

type navigationCompletedEventArgsVtbl struct {
	iUnknownVtbl
	GetIsSuccess      uintptr
	GetWebErrorStatus uintptr
	GetNavigationID   uintptr
}

type newWindowRequestedEventArgs struct {
	vtbl *newWindowRequestedEventArgsVtbl
}

type newWindowRequestedEventArgsVtbl struct {
	iUnknownVtbl
	GetUri             uintptr
	PutNewWindow       uintptr
	GetNewWindow       uintptr
	PutHandled         uintptr
	GetHandled         uintptr
	GetIsUserInitiated uintptr
	GetDeferral        uintptr
	GetWindowFeatures  uintptr
}

type permissionRequestedEventArgs struct {
	vtbl *permissionRequestedEventArgsVtbl
}

type permissionRequestedEventArgsVtbl struct {
	iUnknownVtbl
	GetUri             uintptr
	GetPermissionKind  uintptr
	GetIsUserInitiated uintptr
	GetState           uintptr
	PutState           uintptr
	GetDeferral        uintptr
}

// COREWEBVIEW2_PERMISSION_KIND / _STATE values we act on.
const (
	permissionKindNotifications = 4
	permissionStateAllow        = 1
)

// Navigate loads url.
func (w *CoreWebView2) Navigate(url string) error {
	hr := call(w.vtbl.Navigate, unsafe.Pointer(w), uintptr(unsafe.Pointer(utf16Ptr(url))))
	return checkHR("Navigate", hr)
}

// NavigateToString renders html directly (no server; ~2MB limit).
func (w *CoreWebView2) NavigateToString(html string) error {
	hr := call(w.vtbl.NavigateToString, unsafe.Pointer(w), uintptr(unsafe.Pointer(utf16Ptr(html))))
	return checkHR("NavigateToString", hr)
}

// PostWebMessageAsJson delivers a JSON message to the page.
func (w *CoreWebView2) PostWebMessageAsJson(json string) error {
	hr := call(w.vtbl.PostWebMessageAsJson, unsafe.Pointer(w), uintptr(unsafe.Pointer(utf16Ptr(json))))
	return checkHR("PostWebMessageAsJson", hr)
}

// DocumentTitle returns the current document title.
func (w *CoreWebView2) DocumentTitle() string {
	var p *uint16
	if call(w.vtbl.GetDocumentTitle, unsafe.Pointer(w), uintptr(unsafe.Pointer(&p))) != 0 {
		return ""
	}
	return takeCoTaskString(p)
}

// OpenDevToolsWindow opens the DevTools for this webview.
func (w *CoreWebView2) OpenDevToolsWindow() {
	call(w.vtbl.OpenDevToolsWindow, unsafe.Pointer(w))
}

// AddScriptOnDocumentCreated registers js to run before any page script,
// in every future document. Pumps until registration completes so a
// Navigate immediately after is guaranteed to include it.
func (w *CoreWebView2) AddScriptOnDocumentCreated(js string) error {
	var done bool
	h := newHandler(func(errCode, id unsafe.Pointer) uintptr {
		done = true
		return 0
	})
	defer unpin(h)
	hr := call(w.vtbl.AddScriptToExecuteOnDocumentCreated, unsafe.Pointer(w),
		uintptr(unsafe.Pointer(utf16Ptr(js))), h.ptr())
	if err := checkHR("AddScriptToExecuteOnDocumentCreated", hr); err != nil {
		return err
	}
	return awaitFlag(&done)
}

// OnWebMessageReceived subscribes to messages posted by page script.
// The handler stays registered (and pinned) for the webview's lifetime.
func (w *CoreWebView2) OnWebMessageReceived(fn func(json string)) error {
	h := newHandler(func(sender, args unsafe.Pointer) uintptr {
		a := (*webMessageReceivedEventArgs)(args)
		var p *uint16
		if call(a.vtbl.GetWebMessageAsJson, unsafe.Pointer(a), uintptr(unsafe.Pointer(&p))) == 0 {
			fn(takeCoTaskString(p))
		}
		return 0
	})
	var token int64
	hr := call(w.vtbl.AddWebMessageReceived, unsafe.Pointer(w), h.ptr(), uintptr(unsafe.Pointer(&token)))
	return checkHR("add_WebMessageReceived", hr)
}

// OnDocumentTitleChanged subscribes to title changes.
func (w *CoreWebView2) OnDocumentTitleChanged(fn func(title string)) error {
	h := newHandler(func(sender, args unsafe.Pointer) uintptr {
		fn(w.DocumentTitle())
		return 0
	})
	var token int64
	hr := call(w.vtbl.AddDocumentTitleChanged, unsafe.Pointer(w), h.ptr(), uintptr(unsafe.Pointer(&token)))
	return checkHR("add_DocumentTitleChanged", hr)
}

// OnNavigationCompleted reports every top-level navigation's outcome.
func (w *CoreWebView2) OnNavigationCompleted(fn func(ok bool, webErrorStatus int32)) error {
	h := newHandler(func(sender, args unsafe.Pointer) uintptr {
		a := (*navigationCompletedEventArgs)(args)
		var ok int32
		var status int32
		call(a.vtbl.GetIsSuccess, args, uintptr(unsafe.Pointer(&ok)))
		call(a.vtbl.GetWebErrorStatus, args, uintptr(unsafe.Pointer(&status)))
		fn(ok != 0, status)
		return 0
	})
	var token int64
	hr := call(w.vtbl.AddNavigationCompleted, unsafe.Pointer(w), h.ptr(), uintptr(unsafe.Pointer(&token)))
	return checkHR("add_NavigationCompleted", hr)
}

// ExecuteScript runs js in the page and delivers the result (as JSON) to
// fn. Fire-and-forget when fn is nil.
func (w *CoreWebView2) ExecuteScript(js string, fn func(resultJSON string)) error {
	h := newHandler(func(errCode, result unsafe.Pointer) uintptr {
		if fn != nil && result != nil {
			// The result string is owned by the callee; copy, don't free.
			fn(windows.UTF16PtrToString((*uint16)(result)))
		}
		return 0
	})
	hr := call(w.vtbl.ExecuteScript, unsafe.Pointer(w),
		uintptr(unsafe.Pointer(utf16Ptr(js))), h.ptr())
	return checkHR("ExecuteScript", hr)
}

// OnNewWindowRequested consults fn for every window.open/target=_blank.
// When fn returns true (it opened the URL elsewhere), the popup is
// suppressed; false keeps WebView2's default popup (OAuth flows).
func (w *CoreWebView2) OnNewWindowRequested(fn func(uri string) bool) error {
	h := newHandler(func(sender, args unsafe.Pointer) uintptr {
		a := (*newWindowRequestedEventArgs)(args)
		var p *uint16
		if call(a.vtbl.GetUri, args, uintptr(unsafe.Pointer(&p))) != 0 {
			return 0
		}
		if fn(takeCoTaskString(p)) {
			call(a.vtbl.PutHandled, args, 1)
		}
		return 0
	})
	var token int64
	hr := call(w.vtbl.AddNewWindowRequested, unsafe.Pointer(w), h.ptr(), uintptr(unsafe.Pointer(&token)))
	return checkHR("add_NewWindowRequested", hr)
}

// AutoGrantNotifications answers notification permission prompts with
// ALLOW; other kinds keep the default behaviour.
func (w *CoreWebView2) AutoGrantNotifications() error {
	h := newHandler(func(sender, args unsafe.Pointer) uintptr {
		a := (*permissionRequestedEventArgs)(args)
		var kind int32
		if call(a.vtbl.GetPermissionKind, unsafe.Pointer(a), uintptr(unsafe.Pointer(&kind))) == 0 &&
			kind == permissionKindNotifications {
			call(a.vtbl.PutState, unsafe.Pointer(a), permissionStateAllow)
		}
		return 0
	})
	var token int64
	hr := call(w.vtbl.AddPermissionRequested, unsafe.Pointer(w), h.ptr(), uintptr(unsafe.Pointer(&token)))
	return checkHR("add_PermissionRequested", hr)
}

// ConfigureSettings applies our fixed settings: no status bar, and
// DevTools only when debug is set.
func (w *CoreWebView2) ConfigureSettings(debug bool) error {
	var p unsafe.Pointer
	if err := checkHR("get_Settings", call(w.vtbl.GetSettings, unsafe.Pointer(w), uintptr(unsafe.Pointer(&p)))); err != nil {
		return err
	}
	s := (*settings)(p)
	call(s.vtbl.PutIsStatusBarEnabled, p, 0)
	devtools := uintptr(0)
	if debug {
		devtools = 1
	}
	call(s.vtbl.PutAreDevToolsEnabled, p, devtools)
	release(p)
	return nil
}
