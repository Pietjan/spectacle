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

	GtkHeaderBarNew      func() uintptr
	GtkWindowSetTitlebar func(win, titlebar uintptr)

	GtkFixedNew    func() uintptr
	GtkFixedPut    func(fixed, child uintptr, x, y float64)
	GtkFixedMove   func(fixed, child uintptr, x, y float64)
	GtkFixedRemove func(fixed, child uintptr)

	GtkNativeGetSurface func(native uintptr) uintptr

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
	{&GtkHeaderBarNew, "gtk_header_bar_new"},
	{&GtkWindowSetTitlebar, "gtk_window_set_titlebar"},
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
