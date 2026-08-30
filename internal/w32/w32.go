//go:build windows

// Package w32 holds the raw Win32 surface spectacle uses: lazy proc bindings,
// the handful of structs, and their constants. Nothing here is clever —
// it is the dictionary the rest of the win backend is written with.
package w32

import (
	"golang.org/x/sys/windows"
)

var (
	user32   = windows.NewLazySystemDLL("user32.dll")
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	shell32  = windows.NewLazySystemDLL("shell32.dll")
	ole32    = windows.NewLazySystemDLL("ole32.dll")
	gdi32    = windows.NewLazySystemDLL("gdi32.dll")
	dwmapi   = windows.NewLazySystemDLL("dwmapi.dll")
	dcomp    = windows.NewLazySystemDLL("dcomp.dll")

	RegisterClassEx           = user32.NewProc("RegisterClassExW")
	CreateWindowEx            = user32.NewProc("CreateWindowExW")
	DefWindowProc             = user32.NewProc("DefWindowProcW")
	DestroyWindow             = user32.NewProc("DestroyWindow")
	ShowWindow                = user32.NewProc("ShowWindow")
	IsWindowVisible           = user32.NewProc("IsWindowVisible")
	GetMessage                = user32.NewProc("GetMessageW")
	TranslateMessage          = user32.NewProc("TranslateMessage")
	DispatchMessage           = user32.NewProc("DispatchMessageW")
	PostQuitMessage           = user32.NewProc("PostQuitMessage")
	PostMessage               = user32.NewProc("PostMessageW")
	LoadCursor                = user32.NewProc("LoadCursorW")
	LoadIcon                  = user32.NewProc("LoadIconW")
	GetClientRect             = user32.NewProc("GetClientRect")
	GetWindowRect             = user32.NewProc("GetWindowRect")
	SetWindowPos              = user32.NewProc("SetWindowPos")
	GetDpiForWindow           = user32.NewProc("GetDpiForWindow")
	SetProcessDpiAwarenessCtx = user32.NewProc("SetProcessDpiAwarenessContext")
	SetForegroundWindow       = user32.NewProc("SetForegroundWindow")
	CreatePopupMenu           = user32.NewProc("CreatePopupMenu")
	DestroyMenu               = user32.NewProc("DestroyMenu")
	AppendMenu                = user32.NewProc("AppendMenuW")
	TrackPopupMenu            = user32.NewProc("TrackPopupMenu")
	GetCursorPos              = user32.NewProc("GetCursorPos")
	MessageBox                = user32.NewProc("MessageBoxW")
	GetModuleHandle           = kernel32.NewProc("GetModuleHandleW")
	ShellNotifyIcon           = shell32.NewProc("Shell_NotifyIconW")
	ShellExecute              = shell32.NewProc("ShellExecuteW")
	SetAppUserModelID         = shell32.NewProc("SetCurrentProcessExplicitAppUserModelID")
	CoInitializeEx            = ole32.NewProc("CoInitializeEx")
	CoTaskMemFree             = ole32.NewProc("CoTaskMemFree")
	OleInitialize             = ole32.NewProc("OleInitialize")
	RegisterDragDrop          = ole32.NewProc("RegisterDragDrop")
	RevokeDragDrop            = ole32.NewProc("RevokeDragDrop")
	CreateBitmap              = gdi32.NewProc("CreateBitmap")
	CreateSolidBrush          = gdi32.NewProc("CreateSolidBrush")
	DeleteObject              = gdi32.NewProc("DeleteObject")
	CreateIconIndirect        = user32.NewProc("CreateIconIndirect")
	DestroyIcon               = user32.NewProc("DestroyIcon")
	DwmSetWindowAttribute     = dwmapi.NewProc("DwmSetWindowAttribute")

	// Composition hosting: DirectComposition device creation plus the
	// input plumbing the app-forwarded mouse routing needs.
	DCompositionCreateDevice2 = dcomp.NewProc("DCompositionCreateDevice2")
	SetCapture                = user32.NewProc("SetCapture")
	ReleaseCapture            = user32.NewProc("ReleaseCapture")
	SetCursor                 = user32.NewProc("SetCursor")
	ScreenToClient            = user32.NewProc("ScreenToClient")
	TrackMouseEvent           = user32.NewProc("TrackMouseEvent")
	SetFocus                  = user32.NewProc("SetFocus")
)

const (
	WsOverlappedWindow = 0x00CF0000
	CwUseDefault       = 0x80000000

	SwHide          = 0
	SwShow          = 5
	SwShowNormal    = 1
	SwShowMinimized = 2

	WmDestroy    = 0x0002
	WmMove       = 0x0003
	WmSize       = 0x0005
	WmClose      = 0x0010
	WmSetFocus   = 0x0007
	WmCommand    = 0x0111
	WmDpiChanged = 0x02E0
	WmApp        = 0x8000

	// Mouse messages forwarded to composition-hosted webviews. The
	// COREWEBVIEW2_MOUSE_EVENT_KIND values are these same message codes.
	WmMouseMove     = 0x0200
	WmLButtonDown   = 0x0201
	WmLButtonUp     = 0x0202
	WmLButtonDblClk = 0x0203
	WmRButtonDown   = 0x0204
	WmRButtonUp     = 0x0205
	WmRButtonDblClk = 0x0206
	WmMButtonDown   = 0x0207
	WmMButtonUp     = 0x0208
	WmMButtonDblClk = 0x0209
	WmMouseWheel    = 0x020A
	WmXButtonDown   = 0x020B
	WmXButtonUp     = 0x020C
	WmXButtonDblClk = 0x020D
	WmMouseHWheel   = 0x020E
	WmMouseLeave    = 0x02A3
	WmSetCursor     = 0x0020

	HtClient = 1

	TmeLeave = 0x0002

	// Balloon-notification events (tray callback, version 4).
	NinBalloonUserClick = 0x0405

	IdcArrow       = 32512
	IdiApplication = 32512

	ColorWindow = 5

	CoinitApartmentThreaded = 0x2

	// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 is the pseudo-handle -4.
	DpiAwarenessContextPerMonitorAwareV2 = ^uintptr(3)

	MfString    = 0x0000
	MfSeparator = 0x0800

	TpmReturnCmd   = 0x0100
	TpmRightButton = 0x0002

	NimAdd        = 0
	NimModify     = 1
	NimDelete     = 2
	NimSetVersion = 4

	NifMessage = 0x01
	NifIcon    = 0x02
	NifTip     = 0x04
	NifInfo    = 0x10

	NiifInfo      = 0x01
	NiifUser      = 0x04
	NiifLargeIcon = 0x20

	NotifyIconVersion4 = 4

	SwpNoZOrder   = 0x0004
	SwpNoActivate = 0x0010

	MbIconError = 0x10
	MbOK        = 0x0

	// DWMWINDOWATTRIBUTE values for titlebar styling.
	DwmwaUseImmersiveDarkMode = 20
	DwmwaBorderColor          = 34
	DwmwaCaptionColor         = 35
	DwmwaTextColor            = 36
)

// Point is the Win32 POINT.
type Point struct {
	X, Y int32
}

// Rect is the Win32 RECT.
type Rect struct {
	Left, Top, Right, Bottom int32
}

// Msg is the Win32 MSG.
type Msg struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      Point
	_       uint32 // lPrivate
}

// WndClassEx is the Win32 WNDCLASSEXW.
type WndClassEx struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       uintptr
}

// NotifyIconData is the Win32 NOTIFYICONDATAW.
type NotifyIconData struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
	DwState          uint32
	DwStateMask      uint32
	SzInfo           [256]uint16
	UVersion         uint32
	SzInfoTitle      [64]uint16
	DwInfoFlags      uint32
	GuidItem         windows.GUID
	HBalloonIcon     uintptr
}

// TrackMouseEventData is the Win32 TRACKMOUSEEVENT.
type TrackMouseEventData struct {
	CbSize      uint32
	DwFlags     uint32
	HwndTrack   uintptr
	DwHoverTime uint32
}

// IconInfo is the Win32 ICONINFO.
type IconInfo struct {
	FIcon    int32
	XHotspot uint32
	YHotspot uint32
	HbmMask  uintptr
	HbmColor uintptr
}

// Loword extracts the low 16 bits.
func Loword(v uintptr) uint16 { return uint16(v & 0xffff) }

// Hiword extracts bits 16..31.
func Hiword(v uintptr) uint16 { return uint16((v >> 16) & 0xffff) }
