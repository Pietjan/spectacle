//go:build linux

package native

var jscValueToString func(value uintptr) *byte

var jscFuncs = []registration{
	{&jscValueToString, "jsc_value_to_string"},
}

// JSCValueString converts a JSCValue* to a Go string.
func JSCValueString(value uintptr) string {
	return GoStringOwned(jscValueToString(value))
}
