//go:build linux

package native

import "unsafe"

var (
	GIdleAddFull    func(priority int32, fn uintptr, data uintptr, notify uintptr) uint32
	GMainLoopNew    func(context uintptr, isRunning int32) uintptr
	GMainLoopRun    func(loop uintptr)
	GMainLoopQuit   func(loop uintptr)
	GMainLoopUnref  func(loop uintptr)
	GFree           func(mem unsafe.Pointer)
	gBytesGetData   func(bytes uintptr, size *uint64) *byte
	gBytesGetSize   func(bytes uintptr) uint64
	GBytesUnref     func(bytes uintptr)
	GTimeoutAddFull func(priority int32, interval uint32, fn uintptr, data uintptr, notify uintptr) uint32
	GSetPrgname     func(name string)
)

var glibFuncs = []registration{
	{&GIdleAddFull, "g_idle_add_full"},
	{&GMainLoopNew, "g_main_loop_new"},
	{&GMainLoopRun, "g_main_loop_run"},
	{&GMainLoopQuit, "g_main_loop_quit"},
	{&GMainLoopUnref, "g_main_loop_unref"},
	{&GFree, "g_free"},
	{&gBytesGetData, "g_bytes_get_data"},
	{&gBytesGetSize, "g_bytes_get_size"},
	{&GBytesUnref, "g_bytes_unref"},
	{&GTimeoutAddFull, "g_timeout_add_full"},
	{&GSetPrgname, "g_set_prgname"},
}

// GLib source priorities.
const (
	PriorityDefault     = 0
	PriorityDefaultIdle = 200
)

// BytesCopy copies a GBytes' payload into Go memory (does not unref).
func BytesCopy(bytes uintptr) []byte {
	var size uint64
	data := gBytesGetData(bytes, &size)
	if data == nil || size == 0 {
		return nil
	}
	out := make([]byte, size)
	copy(out, unsafe.Slice(data, size))
	return out
}
