//go:build linux

package native

var (
	GtkInit                 func()
	GtkWindowNew            func() uintptr
	GtkWindowSetTitle       func(win uintptr, title string)
	GtkWindowSetDefaultSize func(win uintptr, w, h int32)
	GtkWindowGetDefaultSize func(win uintptr, w, h *int32)
	GtkWindowSetChild       func(win uintptr, child uintptr)
	GtkWindowPresent        func(win uintptr)
	GtkWindowDestroy        func(win uintptr)
	GtkWindowIsMaximized    func(win uintptr) int32
	GtkWindowSetIconName    func(win uintptr, name string)

	GtkWidgetSetVisible     func(w uintptr, visible int32)
	GtkWidgetGetVisible     func(w uintptr) int32
	GtkWidgetSetSizeRequest func(w uintptr, width, height int32)
	GtkWidgetGrabFocus      func(w uintptr) int32
	GtkWidgetGetScaleFactor func(w uintptr) int32
	GtkWidgetGetWidth       func(w uintptr) int32
	GtkWidgetGetHeight      func(w uintptr) int32
	GtkWidgetRealize        func(w uintptr)
	GtkWidgetGetNative      func(w uintptr) uintptr
	GtkWidgetSetCanTarget   func(w uintptr, canTarget int32)
	GtkWidgetInsertBefore   func(w, parent, nextSibling uintptr)
	GtkWidgetAddController  func(w, controller uintptr)
	// Deprecated in GTK 4.12 but kept exported; the graphene-based
	// replacement needs by-value struct marshalling purego can't do.
	GtkWidgetTranslateCoordinates func(src, dest uintptr, srcX, srcY float64, destX, destY *float64) int32

	GtkHeaderBarNew      func() uintptr
	GtkWindowSetTitlebar func(win, titlebar uintptr)

	GtkScrolledWindowNew       func() uintptr
	GtkScrolledWindowSetChild  func(sw, child uintptr)
	GtkScrolledWindowSetPolicy func(sw uintptr, h, v int32)

	GtkFixedNew    func() uintptr
	GtkFixedPut    func(fixed, child uintptr, x, y float64)
	GtkFixedMove   func(fixed, child uintptr, x, y float64)
	GtkFixedRemove func(fixed, child uintptr)

	GtkNativeGetSurface          func(native uintptr) uintptr
	GtkNativeGetSurfaceTransform func(native uintptr, x, y *float64)

	GtkEventControllerLegacyNew           func() uintptr
	GtkEventControllerSetPropagationPhase func(controller uintptr, phase int32)
	GdkEventGetPosition                   func(event uintptr, x, y *float64) int32

	GtkSettingsGetDefault func() uintptr

	GtkCssProviderNew                    func() uintptr
	GtkCssProviderLoadFromString         func(provider uintptr, css string)
	GtkStyleContextAddProviderForDisplay func(display, provider uintptr, priority uint32)
	GdkDisplayGetDefault                 func() uintptr

	GdkTextureSaveToPngBytes func(texture uintptr) uintptr
)

var gtkFuncs = []registration{
	{&GtkInit, "gtk_init"},
	{&GtkWindowNew, "gtk_window_new"},
	{&GtkWindowSetTitle, "gtk_window_set_title"},
	{&GtkWindowSetDefaultSize, "gtk_window_set_default_size"},
	{&GtkWindowGetDefaultSize, "gtk_window_get_default_size"},
	{&GtkWindowSetChild, "gtk_window_set_child"},
	{&GtkWindowPresent, "gtk_window_present"},
	{&GtkWindowDestroy, "gtk_window_destroy"},
	{&GtkWindowIsMaximized, "gtk_window_is_maximized"},
	{&GtkWindowSetIconName, "gtk_window_set_icon_name"},
	{&GtkWidgetSetVisible, "gtk_widget_set_visible"},
	{&GtkWidgetGetVisible, "gtk_widget_get_visible"},
	{&GtkWidgetSetSizeRequest, "gtk_widget_set_size_request"},
	{&GtkWidgetGrabFocus, "gtk_widget_grab_focus"},
	{&GtkWidgetGetScaleFactor, "gtk_widget_get_scale_factor"},
	{&GtkWidgetGetWidth, "gtk_widget_get_width"},
	{&GtkWidgetGetHeight, "gtk_widget_get_height"},
	{&GtkWidgetRealize, "gtk_widget_realize"},
	{&GtkWidgetGetNative, "gtk_widget_get_native"},
	{&GtkWidgetSetCanTarget, "gtk_widget_set_can_target"},
	{&GtkWidgetInsertBefore, "gtk_widget_insert_before"},
	{&GtkWidgetAddController, "gtk_widget_add_controller"},
	{&GtkWidgetTranslateCoordinates, "gtk_widget_translate_coordinates"},
	{&GtkNativeGetSurfaceTransform, "gtk_native_get_surface_transform"},
	{&GtkEventControllerLegacyNew, "gtk_event_controller_legacy_new"},
	{&GtkEventControllerSetPropagationPhase, "gtk_event_controller_set_propagation_phase"},
	{&GdkEventGetPosition, "gdk_event_get_position"},
	{&GtkHeaderBarNew, "gtk_header_bar_new"},
	{&GtkWindowSetTitlebar, "gtk_window_set_titlebar"},
	{&GtkScrolledWindowNew, "gtk_scrolled_window_new"},
	{&GtkScrolledWindowSetChild, "gtk_scrolled_window_set_child"},
	{&GtkScrolledWindowSetPolicy, "gtk_scrolled_window_set_policy"},
	{&GtkFixedNew, "gtk_fixed_new"},
	{&GtkFixedPut, "gtk_fixed_put"},
	{&GtkFixedMove, "gtk_fixed_move"},
	{&GtkFixedRemove, "gtk_fixed_remove"},
	{&GtkNativeGetSurface, "gtk_native_get_surface"},
	{&GtkSettingsGetDefault, "gtk_settings_get_default"},
	{&GtkCssProviderNew, "gtk_css_provider_new"},
	{&GtkCssProviderLoadFromString, "gtk_css_provider_load_from_string"},
	{&GtkStyleContextAddProviderForDisplay, "gtk_style_context_add_provider_for_display"},
	{&GdkDisplayGetDefault, "gdk_display_get_default"},
	{&GdkTextureSaveToPngBytes, "gdk_texture_save_to_png_bytes"},
}

// GTK_STYLE_PROVIDER_PRIORITY_APPLICATION.
const StyleProviderPriorityApplication = 600

// GTK_PHASE_CAPTURE: event controllers run root-to-target, before the
// target's own handlers.
const PhaseCapture = 1

// GTK_POLICY_EXTERNAL: no scrollbars, scrolling managed by the caller.
const PolicyExternal = 3
