//go:build windows

package webview2

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// IID_ICoreWebView2Profile8, from WebView2.h (profile deletion,
// runtime 1.0.1661.34+, 2023).
var iidProfile8 = windows.GUID{
	Data1: 0xfbf70c2f, Data2: 0xeb1f, Data3: 0x4383,
	Data4: [8]byte{0x85, 0xa0, 0x16, 0x3e, 0x92, 0x04, 0x40, 0x11},
}

type profile8 struct {
	vtbl *profile8Vtbl
}

// profile8Vtbl walks the ICoreWebView2Profile.._8 inheritance chain;
// only Delete at the tail is called, the rest is layout.
type profile8Vtbl struct {
	iUnknownVtbl
	// ICoreWebView2Profile.
	GetProfileName               uintptr
	GetIsInPrivateModeEnabled    uintptr
	GetProfilePath               uintptr
	GetDefaultDownloadFolderPath uintptr
	PutDefaultDownloadFolderPath uintptr
	GetPreferredColorScheme      uintptr
	PutPreferredColorScheme      uintptr
	// ICoreWebView2Profile2.
	ClearBrowsingData            uintptr
	ClearBrowsingDataInTimeRange uintptr
	ClearBrowsingDataAll         uintptr
	// ICoreWebView2Profile3.
	GetPreferredTrackingPreventionLevel uintptr
	PutPreferredTrackingPreventionLevel uintptr
	// ICoreWebView2Profile4.
	SetPermissionState              uintptr
	GetNonDefaultPermissionSettings uintptr
	// ICoreWebView2Profile5.
	GetCookieManager uintptr
	// ICoreWebView2Profile6.
	GetIsPasswordAutosaveEnabled uintptr
	PutIsPasswordAutosaveEnabled uintptr
	GetIsGeneralAutofillEnabled  uintptr
	PutIsGeneralAutofillEnabled  uintptr
	// ICoreWebView2Profile7.
	AddBrowserExtension  uintptr
	GetBrowserExtensions uintptr
	// ICoreWebView2Profile8.
	Delete        uintptr
	AddDeleted    uintptr
	RemoveDeleted uintptr
}

// DeleteProfile marks this webview's browsing profile for deletion; its
// data is removed from disk once the profile's webviews are closed.
// Errors out on runtimes older than the profile-deletion API.
func (w *CoreWebView2) DeleteProfile() error {
	p, err := queryInterface(unsafe.Pointer(w), &iidCoreWebView2_15)
	if err != nil {
		return err
	}
	defer release(p)
	v15 := (*coreWebView2_15)(p)
	var prof unsafe.Pointer
	if err := checkHR("get_Profile", call(v15.vtbl.GetProfile, p, uintptr(unsafe.Pointer(&prof)))); err != nil {
		return err
	}
	defer release(prof)
	p8, err := queryInterface(prof, &iidProfile8)
	if err != nil {
		return err
	}
	defer release(p8)
	return checkHR("Profile.Delete", call((*profile8)(p8).vtbl.Delete, p8))
}
