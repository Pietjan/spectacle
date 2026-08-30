//go:build linux

package native

import "unsafe"

// C string returns are typed *byte at the binding layer, so no uintptr
// round trips ever occur in Go code.

// GoString copies a borrowed NUL-terminated C string.
func GoString(p *byte) string {
	if p == nil {
		return ""
	}
	var n int
	for *(*byte)(unsafe.Add(unsafe.Pointer(p), n)) != 0 {
		n++
	}
	return string(unsafe.Slice(p, n))
}

// GoStringOwned copies a caller-owned C string and g_free's it.
func GoStringOwned(p *byte) string {
	if p == nil {
		return ""
	}
	s := GoString(p)
	GFree(unsafe.Pointer(p))
	return s
}
