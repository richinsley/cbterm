//go:build darwin

package darwin

/*
#include "bridge.h"
#include <stdlib.h>
*/
import "C"

import (
	"unsafe"
)

// Window represents a native macOS window.
type Window struct {
	ptr     C.CBTermWindow
	webview *WebView
}

func newWindow(opts WindowOptions, debug bool) (*Window, error) {
	title := C.CString(opts.Title)
	defer C.free(unsafe.Pointer(title))

	ptr := C.cbterm_window_new(
		title,
		C.int(opts.Width),
		C.int(opts.Height),
		C.int(opts.X),
		C.int(opts.Y),
		C.bool(opts.Resizable),
		C.bool(opts.Frameless),
	)

	w := &Window{ptr: ptr}

	// Get the webview from the window
	wvPtr := C.cbterm_window_get_webview(ptr)
	w.webview = &WebView{
		ptr:   C.CBTermWebView(wvPtr),
		debug: debug,
	}

	return w, nil
}

// Show makes the window visible.
func (w *Window) Show() {
	C.cbterm_window_show(w.ptr)
}

// Hide makes the window invisible.
func (w *Window) Hide() {
	C.cbterm_window_hide(w.ptr)
}

// Close closes and destroys the window.
func (w *Window) Close() {
	C.cbterm_window_close(w.ptr)
}

// Focus brings the window to front.
func (w *Window) Focus() {
	C.cbterm_window_focus(w.ptr)
}

// Center centers the window on screen.
func (w *Window) Center() {
	C.cbterm_window_center(w.ptr)
}

// SetTitle sets the window title.
func (w *Window) SetTitle(title string) {
	t := C.CString(title)
	defer C.free(unsafe.Pointer(t))
	C.cbterm_window_set_title(w.ptr, t)
}

// SetSize sets the window dimensions.
func (w *Window) SetSize(width, height int) {
	C.cbterm_window_set_size(w.ptr, C.int(width), C.int(height))
}

// SetPosition sets the window position.
func (w *Window) SetPosition(x, y int) {
	C.cbterm_window_set_position(w.ptr, C.int(x), C.int(y))
}

// SetMinSize sets the minimum window dimensions.
func (w *Window) SetMinSize(width, height int) {
	C.cbterm_window_set_min_size(w.ptr, C.int(width), C.int(height))
}

// SetMaxSize sets the maximum window dimensions.
func (w *Window) SetMaxSize(width, height int) {
	C.cbterm_window_set_max_size(w.ptr, C.int(width), C.int(height))
}

// GetWebView returns the window's web view.
func (w *Window) GetWebView() *WebView {
	return w.webview
}

// OnClose registers a callback for when the window is about to close.
func (w *Window) OnClose(fn func() bool) {
	cb := registerCallback(fn)
	C.cbterm_window_set_close_callback(w.ptr, cb)
}
