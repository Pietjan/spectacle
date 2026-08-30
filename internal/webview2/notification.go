//go:build windows

package webview2

import (
	"log"
	"unsafe"

	"golang.org/x/sys/windows"
)

// IID_ICoreWebView2_24, from WebView2.h (NotificationReceived API,
// runtime 1.0.2478.35+, 2024).
var iidCoreWebView2_24 = windows.GUID{
	Data1: 0x39a7ad55, Data2: 0x4287, Data3: 0x5cc1,
	Data4: [8]byte{0x88, 0xa1, 0xc6, 0xf4, 0x58, 0x59, 0x38, 0x24},
}

type coreWebView2_24 struct {
	vtbl *coreWebView2_24Vtbl
}

type coreWebView2_24Vtbl struct {
	coreWebView2_19Vtbl
	// ICoreWebView2_20.
	GetFrameID uintptr
	// ICoreWebView2_21.
	ExecuteScriptWithResult uintptr
	// ICoreWebView2_22.
	AddWebResourceRequestedFilterWithRequestSourceKinds    uintptr
	RemoveWebResourceRequestedFilterWithRequestSourceKinds uintptr
	// ICoreWebView2_23.
	PostWebMessageAsJsonWithAdditionalObjects uintptr
	// ICoreWebView2_24.
	AddNotificationReceived    uintptr
	RemoveNotificationReceived uintptr
}

type notificationReceivedEventArgs struct {
	vtbl *notificationReceivedEventArgsVtbl
}

type notificationReceivedEventArgsVtbl struct {
	iUnknownVtbl
	GetSenderOrigin uintptr
	GetNotification uintptr
	PutHandled      uintptr
	GetHandled      uintptr
	GetDeferral     uintptr
}

type notification struct {
	vtbl *notificationVtbl
}

type notificationVtbl struct {
	iUnknownVtbl
	AddCloseRequested      uintptr
	RemoveCloseRequested   uintptr
	ReportShown            uintptr
	ReportClicked          uintptr
	ReportClosed           uintptr
	GetBody                uintptr
	GetDirection           uintptr
	GetLanguage            uintptr
	GetTag                 uintptr
	GetIconURI             uintptr
	GetTitle               uintptr
	GetBadgeURI            uintptr
	GetBodyImageURI        uintptr
	GetShouldRenotify      uintptr
	GetRequiresInteraction uintptr
	GetIsSilent            uintptr
	GetTimestamp           uintptr
	GetVibrationPattern    uintptr
}

// OnNotificationReceived subscribes to web notifications — page and
// service-worker alike — marking them handled so WebView2 shows nothing
// itself. Errors out on runtimes older than 2024's notification API.
func (w *CoreWebView2) OnNotificationReceived(fn func(title, body string)) error {
	p, err := queryInterface(unsafe.Pointer(w), &iidCoreWebView2_24)
	if err != nil {
		return err
	}
	// p stays referenced for the webview's lifetime via the handler.
	v24 := (*coreWebView2_24)(p)
	h := newHandler(func(sender, argsPtr unsafe.Pointer) uintptr {
		log.Printf("webview2: notification received event")
		args := (*notificationReceivedEventArgs)(argsPtr)
		call(args.vtbl.PutHandled, argsPtr, 1)
		var np unsafe.Pointer
		if call(args.vtbl.GetNotification, argsPtr, uintptr(unsafe.Pointer(&np))) != 0 || np == nil {
			return 0
		}
		n := (*notification)(np)
		var title, body string
		var s *uint16
		if call(n.vtbl.GetTitle, np, uintptr(unsafe.Pointer(&s))) == 0 {
			title = takeCoTaskString(s)
		}
		if call(n.vtbl.GetBody, np, uintptr(unsafe.Pointer(&s))) == 0 {
			body = takeCoTaskString(s)
		}
		call(n.vtbl.ReportShown, np)
		release(np)
		fn(title, body)
		return 0
	})
	var token int64
	hr := call(v24.vtbl.AddNotificationReceived, p, h.ptr(), uintptr(unsafe.Pointer(&token)))
	return checkHR("add_NotificationReceived", hr)
}
