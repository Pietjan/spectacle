//go:build windows

package webview2

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// IID_ICoreWebView2_15, from WebView2.h. The interface that carries the
// favicon API (runtime 1.0.1518.46+, mid-2023; evergreen in practice).
var iidCoreWebView2_15 = windows.GUID{
	Data1: 0x517b2d1d, Data2: 0x7dae, Data3: 0x4a66,
	Data4: [8]byte{0xa4, 0xf4, 0x10, 0x35, 0x2f, 0xfb, 0x95, 0x18},
}

type coreWebView2_15 struct {
	vtbl *coreWebView2_15Vtbl
}

// coreWebView2_15Vtbl walks the whole ICoreWebView2.._15 inheritance
// chain; only the _15 tail is called, the rest is layout.
type coreWebView2_15Vtbl struct {
	coreWebView2Vtbl
	// ICoreWebView2_2.
	AddWebResourceResponseReceived    uintptr
	RemoveWebResourceResponseReceived uintptr
	NavigateWithWebResourceRequest    uintptr
	AddDOMContentLoaded               uintptr
	RemoveDOMContentLoaded            uintptr
	GetCookieManager                  uintptr
	GetEnvironment                    uintptr
	// ICoreWebView2_3.
	TrySuspend                          uintptr
	Resume                              uintptr
	GetIsSuspended                      uintptr
	SetVirtualHostNameToFolderMapping   uintptr
	ClearVirtualHostNameToFolderMapping uintptr
	// ICoreWebView2_4.
	AddFrameCreated        uintptr
	RemoveFrameCreated     uintptr
	AddDownloadStarting    uintptr
	RemoveDownloadStarting uintptr
	// ICoreWebView2_5.
	AddClientCertificateRequested    uintptr
	RemoveClientCertificateRequested uintptr
	// ICoreWebView2_6.
	OpenTaskManagerWindow uintptr
	// ICoreWebView2_7.
	PrintToPdf uintptr
	// ICoreWebView2_8.
	AddIsMutedChanged                   uintptr
	RemoveIsMutedChanged                uintptr
	GetIsMuted                          uintptr
	PutIsMuted                          uintptr
	AddIsDocumentPlayingAudioChanged    uintptr
	RemoveIsDocumentPlayingAudioChanged uintptr
	GetIsDocumentPlayingAudio           uintptr
	// ICoreWebView2_9.
	AddIsDefaultDownloadDialogOpenChanged    uintptr
	RemoveIsDefaultDownloadDialogOpenChanged uintptr
	GetIsDefaultDownloadDialogOpen           uintptr
	OpenDefaultDownloadDialog                uintptr
	CloseDefaultDownloadDialog               uintptr
	GetDefaultDownloadDialogCornerAlignment  uintptr
	PutDefaultDownloadDialogCornerAlignment  uintptr
	GetDefaultDownloadDialogMargin           uintptr
	PutDefaultDownloadDialogMargin           uintptr
	// ICoreWebView2_10.
	AddBasicAuthenticationRequested    uintptr
	RemoveBasicAuthenticationRequested uintptr
	// ICoreWebView2_11.
	CallDevToolsProtocolMethodForSession uintptr
	AddContextMenuRequested              uintptr
	RemoveContextMenuRequested           uintptr
	// ICoreWebView2_12.
	AddStatusBarTextChanged    uintptr
	RemoveStatusBarTextChanged uintptr
	GetStatusBarText           uintptr
	// ICoreWebView2_13.
	GetProfile uintptr
	// ICoreWebView2_14.
	AddServerCertificateErrorDetected    uintptr
	RemoveServerCertificateErrorDetected uintptr
	ClearServerCertificateErrorActions   uintptr
	// ICoreWebView2_15.
	AddFaviconChanged    uintptr
	RemoveFaviconChanged uintptr
	GetFaviconUri        uintptr
	GetFavicon           uintptr
}

// faviconFormatPNG is COREWEBVIEW2_FAVICON_IMAGE_FORMAT_PNG.
const faviconFormatPNG = 0

// iStream is the prefix of COM IStream we need: just Read.
type iStream struct {
	vtbl *iStreamVtbl
}

type iStreamVtbl struct {
	iUnknownVtbl
	Read uintptr
}

// OnFaviconChanged subscribes to favicon changes and delivers the icon
// as PNG bytes. Errors out on runtimes older than the favicon API.
func (w *CoreWebView2) OnFaviconChanged(fn func(png []byte)) error {
	p, err := queryInterface(unsafe.Pointer(w), &iidCoreWebView2_15)
	if err != nil {
		return err
	}
	// p stays referenced for the webview's lifetime via the handler.
	v15 := (*coreWebView2_15)(p)
	h := newHandler(func(sender, args unsafe.Pointer) uintptr {
		fetchFavicon(v15, fn)
		return 0
	})
	var token int64
	hr := call(v15.vtbl.AddFaviconChanged, p, h.ptr(), uintptr(unsafe.Pointer(&token)))
	return checkHR("add_FaviconChanged", hr)
}

func fetchFavicon(v15 *coreWebView2_15, fn func(png []byte)) {
	var h *comHandler
	h = newHandler(func(errCode, stream unsafe.Pointer) uintptr {
		defer unpin(h)
		if int32(uintptr(errCode)) >= 0 && stream != nil {
			if png := readStream((*iStream)(stream)); len(png) > 0 {
				fn(png)
			}
		}
		return 0
	})
	hr := call(v15.vtbl.GetFavicon, unsafe.Pointer(v15), faviconFormatPNG, h.ptr())
	if int32(hr) < 0 {
		unpin(h)
	}
}

// readStream drains a COM IStream. The stream is borrowed; not released.
func readStream(s *iStream) []byte {
	var out []byte
	buf := make([]byte, 4096)
	for {
		var read uint32
		hr := call(s.vtbl.Read, unsafe.Pointer(s),
			uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)),
			uintptr(unsafe.Pointer(&read)))
		if read > 0 {
			out = append(out, buf[:read]...)
		}
		// S_OK with a full buffer means more; anything else ends it.
		if int32(hr) != 0 || read == 0 {
			return out
		}
	}
}
