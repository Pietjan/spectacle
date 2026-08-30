//go:build linux

package native

var (
	gSettingsSchemaSourceGetDefault func() uintptr
	gSettingsSchemaSourceLookup     func(source uintptr, schemaID string, recursive int32) uintptr
	gSettingsSchemaUnref            func(schema uintptr)
	gSettingsNew                    func(schemaID string) uintptr
	gSettingsGetString              func(settings uintptr, key string) *byte
)

var gioFuncs = []registration{
	{&gSettingsSchemaSourceGetDefault, "g_settings_schema_source_get_default"},
	{&gSettingsSchemaSourceLookup, "g_settings_schema_source_lookup"},
	{&gSettingsSchemaUnref, "g_settings_schema_unref"},
	{&gSettingsNew, "g_settings_new"},
	{&gSettingsGetString, "g_settings_get_string"},
}

// SettingsString reads a string key from a GSettings schema, or ""
// when the schema is absent. The schema lookup guard is mandatory:
// g_settings_new aborts the process on a missing schema.
func SettingsString(schemaID, key string) string {
	source := gSettingsSchemaSourceGetDefault()
	if source == 0 {
		return ""
	}
	schema := gSettingsSchemaSourceLookup(source, schemaID, 1)
	if schema == 0 {
		return ""
	}
	gSettingsSchemaUnref(schema)
	settings := gSettingsNew(schemaID)
	if settings == 0 {
		return ""
	}
	defer GObjectUnref(settings)
	return GoStringOwned(gSettingsGetString(settings, key))
}
