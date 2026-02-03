//go:build darwin

package darwin

/*
#include "bridge.h"
#include <stdlib.h>
*/
import "C"

import (
	"errors"
	"strings"
	"unsafe"
)

// WebView represents a macOS WKWebView.
type WebView struct {
	ptr   C.CBTermWebView
	debug bool
}

// LoadURL loads a URL into the web view.
func (wv *WebView) LoadURL(url string) {
	u := C.CString(url)
	defer C.free(unsafe.Pointer(u))
	C.cbterm_webview_load_url(wv.ptr, u)
}

// LoadHTML loads HTML content directly into the web view.
func (wv *WebView) LoadHTML(html string, baseURL string) {
	h := C.CString(html)
	defer C.free(unsafe.Pointer(h))

	var b *C.char
	if baseURL != "" {
		b = C.CString(baseURL)
		defer C.free(unsafe.Pointer(b))
	}

	C.cbterm_webview_load_html(wv.ptr, h, b)
}

// Eval executes JavaScript and returns the result.
func (wv *WebView) Eval(js string) (string, error) {
	j := C.CString(js)
	defer C.free(unsafe.Pointer(j))

	result := C.cbterm_webview_eval(wv.ptr, j)
	defer C.cbterm_webview_free_string(result)

	r := C.GoString(result)
	if strings.HasPrefix(r, "error: ") {
		return "", errors.New(strings.TrimPrefix(r, "error: "))
	}
	return r, nil
}

// Bind exposes a Go function to JavaScript.
func (wv *WebView) Bind(name string, fn interface{}) error {
	n := C.CString(name)
	defer C.free(unsafe.Pointer(n))

	// Wrap the function to handle the callback
	wrapper := func(argsJSON string) {
		// TODO: Parse argsJSON and call fn with proper arguments
		if f, ok := fn.(func()); ok {
			f()
		} else if f, ok := fn.(func(string)); ok {
			f(argsJSON)
		}
	}

	cb := registerCallback(wrapper)
	C.cbterm_webview_bind(wv.ptr, n, cb)
	return nil
}

// OnMessage registers a handler for messages from JavaScript.
func (wv *WebView) OnMessage(fn func(msg string)) {
	cb := registerCallback(fn)
	C.cbterm_webview_set_message_handler(wv.ptr, cb)
}

// OnNavigate registers a handler for navigation events.
func (wv *WebView) OnNavigate(fn func(url string) bool) {
	cb := registerCallback(fn)
	C.cbterm_webview_set_navigation_handler(wv.ptr, cb)
}

// Reload reloads the current page.
func (wv *WebView) Reload() {
	C.cbterm_webview_reload(wv.ptr)
}

// SetDevToolsEnabled enables or disables developer tools.
func (wv *WebView) SetDevToolsEnabled(enabled bool) {
	C.cbterm_webview_set_devtools(wv.ptr, C.bool(enabled))
}
