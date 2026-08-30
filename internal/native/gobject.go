//go:build linux

package native

import "unsafe"

// GValue mirrors the 24-byte C GValue.
type GValue struct {
	GType uintptr
	Data  [2]uint64
}

// Fundamental GTypes (constant tags, GLib ABI).
const (
	gTypeBoolean uintptr = 5 << 2
	gTypeString  uintptr = 16 << 2
	gTypeObject  uintptr = 20 << 2
)

var (
	gObjectNewWithProperties func(gtype uintptr, n uint32, names *uintptr, values *GValue) uintptr
	GObjectRefSink           func(obj uintptr) uintptr
	GObjectRef               func(obj uintptr) uintptr
	GObjectUnref             func(obj uintptr)
	GObjectSetProperty       func(obj uintptr, name string, value *GValue)
	GSignalConnectData       func(instance uintptr, signal string, handler uintptr, data uintptr, destroy uintptr, flags uint32) uint64
	GSignalHandlerDisconnect func(instance uintptr, id uint64)
	GTypeCheckInstanceIsA    func(instance uintptr, gtype uintptr) int32
	gValueInit               func(v *GValue, gtype uintptr) *GValue
	gValueUnset              func(v *GValue)
	gValueSetString          func(v *GValue, s string)
	gValueSetObject          func(v *GValue, obj uintptr)
	gValueSetBoolean         func(v *GValue, b int32)
)

var gobjectFuncs = []registration{
	{&gObjectNewWithProperties, "g_object_new_with_properties"},
	{&GObjectRefSink, "g_object_ref_sink"},
	{&GObjectRef, "g_object_ref"},
	{&GObjectUnref, "g_object_unref"},
	{&GObjectSetProperty, "g_object_set_property"},
	{&GSignalConnectData, "g_signal_connect_data"},
	{&GSignalHandlerDisconnect, "g_signal_handler_disconnect"},
	{&GTypeCheckInstanceIsA, "g_type_check_instance_is_a"},
	{&gValueInit, "g_value_init"},
	{&gValueUnset, "g_value_unset"},
	{&gValueSetString, "g_value_set_string"},
	{&gValueSetObject, "g_value_set_object"},
	{&gValueSetBoolean, "g_value_set_boolean"},
}

// Prop is one construct property for ObjectNew.
type Prop struct {
	Name string
	// Exactly one of these is used.
	Object uintptr
	String *string
	Bool   *bool
}

// ObjectNew is the no-variadics replacement for g_object_new: construct
// an instance with properties via g_object_new_with_properties.
func ObjectNew(gtype uintptr, props []Prop) uintptr {
	if len(props) == 0 {
		return gObjectNewWithProperties(gtype, 0, nil, nil)
	}
	names := make([]uintptr, len(props))
	values := make([]GValue, len(props))
	cstrs := make([][]byte, len(props)) // keep NUL-terminated names alive
	for i, p := range props {
		cstrs[i] = append([]byte(p.Name), 0)
		names[i] = uintptr(unsafe.Pointer(&cstrs[i][0]))
		switch {
		case p.String != nil:
			gValueInit(&values[i], gTypeString)
			gValueSetString(&values[i], *p.String)
		case p.Bool != nil:
			gValueInit(&values[i], gTypeBoolean)
			b := int32(0)
			if *p.Bool {
				b = 1
			}
			gValueSetBoolean(&values[i], b)
		default:
			gValueInit(&values[i], gTypeObject)
			gValueSetObject(&values[i], p.Object)
		}
	}
	obj := gObjectNewWithProperties(gtype, uint32(len(props)), &names[0], &values[0])
	for i := range values {
		gValueUnset(&values[i])
	}
	return obj
}

// SetBoolProperty sets a boolean GObject property by name.
func SetBoolProperty(obj uintptr, name string, value bool) {
	var v GValue
	gValueInit(&v, gTypeBoolean)
	b := int32(0)
	if value {
		b = 1
	}
	gValueSetBoolean(&v, b)
	GObjectSetProperty(obj, name, &v)
	gValueUnset(&v)
}
