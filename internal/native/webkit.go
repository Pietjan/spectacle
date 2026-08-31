//go:build linux

package native

// GdkRGBA is four C floats.
type GdkRGBA struct {
	R, G, B, A float32
}

var (
	WebkitWebViewGetType                       func() uintptr
	WebkitNetworkSessionNew                    func(dataDir, cacheDir string) uintptr
	WebkitNetworkSessionGetWebsiteDataManager  func(session uintptr) uintptr
	WebkitNetworkSessionGetCookieManager       func(session uintptr) uintptr
	WebkitCookieManagerSetPersistentStorage    func(cm uintptr, filename string, storage int32)
	WebkitWebsiteDataManagerSetFaviconsEnabled func(dm uintptr, enabled int32)
	WebkitUserContentManagerNew                func() uintptr

	WebkitUserContentManagerAddScript                    func(ucm, script uintptr)
	WebkitUserScriptNew                                  func(source string, frames int32, injectTime int32, allowList, blockList uintptr) uintptr
	WebkitUserScriptUnref                                func(script uintptr)
	WebkitUserContentManagerRegisterScriptMessageHandler func(ucm uintptr, name string, worldName uintptr) int32
	WebkitWebViewEvaluateJavascript                      func(view uintptr, script string, length int64, worldName, sourceURI, cancellable, callback, userData uintptr)

	WebkitWebViewLoadURI            func(view uintptr, uri string)
	WebkitWebViewLoadHTML           func(view uintptr, content string, baseURI uintptr)
	WebkitWebViewGetTitle           func(view uintptr) *byte
	WebkitWebViewGetFavicon         func(view uintptr) uintptr
	WebkitWebViewSetBackgroundColor func(view uintptr, rgba *GdkRGBA)
	WebkitWebViewTryClose           func(view uintptr)

	WebkitWebViewGetSettings                            func(view uintptr) uintptr
	WebkitSettingsSetEnableDeveloperExtras              func(settings uintptr, enabled int32)
	WebkitSettingsSetEnableWriteConsoleMessagesToStdout func(settings uintptr, enabled int32)
	WebkitSettingsSetUserAgent                          func(settings uintptr, ua string)

	WebkitNotificationGetTitle                 func(n uintptr) *byte
	WebkitNotificationGetBody                  func(n uintptr) *byte
	WebkitNotificationPermissionRequestGetType func() uintptr
	WebkitPermissionRequestAllow               func(req uintptr)

	WebkitNavigationActionGetRequest func(action uintptr) uintptr
	WebkitURIRequestGetURI           func(req uintptr) *byte
)

var webkitFuncs = []registration{
	{&WebkitWebViewGetType, "webkit_web_view_get_type"},
	{&WebkitNetworkSessionNew, "webkit_network_session_new"},
	{&WebkitNetworkSessionGetWebsiteDataManager, "webkit_network_session_get_website_data_manager"},
	{&WebkitNetworkSessionGetCookieManager, "webkit_network_session_get_cookie_manager"},
	{&WebkitCookieManagerSetPersistentStorage, "webkit_cookie_manager_set_persistent_storage"},
	{&WebkitWebsiteDataManagerSetFaviconsEnabled, "webkit_website_data_manager_set_favicons_enabled"},
	{&WebkitUserContentManagerNew, "webkit_user_content_manager_new"},
	{&WebkitUserContentManagerAddScript, "webkit_user_content_manager_add_script"},
	{&WebkitUserScriptNew, "webkit_user_script_new"},
	{&WebkitUserScriptUnref, "webkit_user_script_unref"},
	{&WebkitUserContentManagerRegisterScriptMessageHandler, "webkit_user_content_manager_register_script_message_handler"},
	{&WebkitWebViewEvaluateJavascript, "webkit_web_view_evaluate_javascript"},
	{&WebkitWebViewLoadURI, "webkit_web_view_load_uri"},
	{&WebkitWebViewLoadHTML, "webkit_web_view_load_html"},
	{&WebkitWebViewGetTitle, "webkit_web_view_get_title"},
	{&WebkitWebViewGetFavicon, "webkit_web_view_get_favicon"},
	{&WebkitWebViewSetBackgroundColor, "webkit_web_view_set_background_color"},
	{&WebkitWebViewTryClose, "webkit_web_view_try_close"},
	{&WebkitWebViewGetSettings, "webkit_web_view_get_settings"},
	{&WebkitSettingsSetEnableDeveloperExtras, "webkit_settings_set_enable_developer_extras"},
	{&WebkitSettingsSetEnableWriteConsoleMessagesToStdout, "webkit_settings_set_enable_write_console_messages_to_stdout"},
	{&WebkitSettingsSetUserAgent, "webkit_settings_set_user_agent"},
	{&WebkitNotificationGetTitle, "webkit_notification_get_title"},
	{&WebkitNotificationGetBody, "webkit_notification_get_body"},
	{&WebkitNotificationPermissionRequestGetType, "webkit_notification_permission_request_get_type"},
	{&WebkitPermissionRequestAllow, "webkit_permission_request_allow"},
	{&WebkitNavigationActionGetRequest, "webkit_navigation_action_get_request"},
	{&WebkitURIRequestGetURI, "webkit_uri_request_get_uri"},
}

// webkit_user_script_new enums.
const (
	UserContentInjectAllFrames      = 0
	UserScriptInjectAtDocumentStart = 0
)

// webkit_cookie_manager_set_persistent_storage enums.
const (
	CookiePersistentStorageSQLite = 1
)
