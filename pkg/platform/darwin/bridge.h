// bridge.h - Objective-C bridge for cbterm platform layer
// This header declares C functions that bridge Go and Objective-C.

#ifndef CBTERM_BRIDGE_H
#define CBTERM_BRIDGE_H

#include <stdint.h>
#include <stdbool.h>

#ifdef __cplusplus
extern "C" {
#endif

// Application lifecycle
void cbterm_app_init(const char* name, bool debug);
void cbterm_app_run(void);
void cbterm_app_quit(void);
void cbterm_app_invoke(void* callback);

// Window management
typedef void* CBTermWindow;

CBTermWindow cbterm_window_new(const char* title, int width, int height, 
                                int x, int y, bool resizable, bool frameless);
void cbterm_window_show(CBTermWindow w);
void cbterm_window_hide(CBTermWindow w);
void cbterm_window_close(CBTermWindow w);
void cbterm_window_focus(CBTermWindow w);
void cbterm_window_center(CBTermWindow w);
void cbterm_window_set_title(CBTermWindow w, const char* title);
void cbterm_window_set_size(CBTermWindow w, int width, int height);
void cbterm_window_set_position(CBTermWindow w, int x, int y);
void cbterm_window_set_min_size(CBTermWindow w, int width, int height);
void cbterm_window_set_max_size(CBTermWindow w, int width, int height);
void* cbterm_window_get_webview(CBTermWindow w);
void cbterm_window_set_close_callback(CBTermWindow w, void* callback);

// WebView
typedef void* CBTermWebView;

void cbterm_webview_load_url(CBTermWebView wv, const char* url);
void cbterm_webview_load_html(CBTermWebView wv, const char* html, const char* base_url);
const char* cbterm_webview_eval(CBTermWebView wv, const char* js);
void cbterm_webview_free_string(const char* str);
void cbterm_webview_bind(CBTermWebView wv, const char* name, void* callback);
void cbterm_webview_set_message_handler(CBTermWebView wv, void* callback);
void cbterm_webview_set_navigation_handler(CBTermWebView wv, void* callback);
void cbterm_webview_reload(CBTermWebView wv);
void cbterm_webview_set_devtools(CBTermWebView wv, bool enabled);

// Menus
typedef void* CBTermMenu;

CBTermMenu cbterm_menu_new(void);
void cbterm_menu_add_item(CBTermMenu m, const char* label, const char* shortcut, 
                          bool enabled, bool checked, void* callback);
void cbterm_menu_add_separator(CBTermMenu m);
void cbterm_menu_add_submenu(CBTermMenu m, const char* label, CBTermMenu submenu);
void cbterm_menu_clear(CBTermMenu m);
void cbterm_menu_popup(CBTermMenu m, int x, int y);
void cbterm_app_set_menu(CBTermMenu m);

#ifdef __cplusplus
}
#endif

#endif // CBTERM_BRIDGE_H
