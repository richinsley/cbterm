//go:build darwin

// Package darwin provides the macOS implementation of the platform layer.
package darwin

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa -framework WebKit

#include "bridge.h"
#include <stdlib.h>
*/
import "C"

import (
	"sync"
	"unsafe"
)

// Options configures the platform layer.
type Options struct {
	Name       string
	WebKitPath string
	Debug      bool
}

// WindowOptions configures a new window.
type WindowOptions struct {
	Title       string
	Width       int
	Height      int
	X           int
	Y           int
	Resizable   bool
	Frameless   bool
	AlwaysOnTop bool
}

// MenuItem represents a menu item.
type MenuItem struct {
	Label    string
	Shortcut string
	Enabled  bool
	Checked  bool
	Action   func()
	Submenu  *Menu
}

// Platform is the macOS platform implementation.
type Platform struct {
	opts     Options
	invokeID int64
	invokeMu sync.Mutex
	invokeWg sync.WaitGroup
}

// New creates a new macOS platform instance.
func New(opts Options) (*Platform, error) {
	p := &Platform{opts: opts}

	name := C.CString(opts.Name)
	defer C.free(unsafe.Pointer(name))

	C.cbterm_app_init(name, C.bool(opts.Debug))

	return p, nil
}

// Run starts the macOS event loop.
func (p *Platform) Run() error {
	C.cbterm_app_run()
	return nil
}

// Quit terminates the application.
func (p *Platform) Quit() {
	C.cbterm_app_quit()
}

// NewWindow creates a new native window.
func (p *Platform) NewWindow(opts WindowOptions) (*Window, error) {
	return newWindow(opts, p.opts.Debug)
}

// NewMenu creates a new menu.
func (p *Platform) NewMenu() *Menu {
	return newMenu()
}

// SetAppMenu sets the application menu bar.
func (p *Platform) SetAppMenu(menu *Menu) {
	C.cbterm_app_set_menu(menu.ptr)
}

// Invoke runs a function on the main thread.
func (p *Platform) Invoke(fn func()) {
	cb := registerCallback(fn)
	C.cbterm_app_invoke(cb)
}

// Callback registry for Go functions called from Objective-C
var (
	callbackMu  sync.Mutex
	callbackID  int64
	callbackMap = make(map[int64]interface{})
)

func registerCallback(fn interface{}) unsafe.Pointer {
	callbackMu.Lock()
	defer callbackMu.Unlock()
	callbackID++
	id := callbackID
	callbackMap[id] = fn
	return unsafe.Pointer(uintptr(id))
}

func getCallback(ptr unsafe.Pointer) interface{} {
	callbackMu.Lock()
	defer callbackMu.Unlock()
	id := int64(uintptr(ptr))
	return callbackMap[id]
}

func unregisterCallback(ptr unsafe.Pointer) {
	callbackMu.Lock()
	defer callbackMu.Unlock()
	id := int64(uintptr(ptr))
	delete(callbackMap, id)
}

//export goInvokeCallback
func goInvokeCallback(ptr unsafe.Pointer) {
	if fn, ok := getCallback(ptr).(func()); ok {
		fn()
	}
	unregisterCallback(ptr)
}

//export goWindowCloseCallback
func goWindowCloseCallback(ptr unsafe.Pointer) C.bool {
	if fn, ok := getCallback(ptr).(func() bool); ok {
		return C.bool(fn())
	}
	return C.bool(true)
}

//export goMessageCallback
func goMessageCallback(ptr unsafe.Pointer, msg *C.char) {
	if fn, ok := getCallback(ptr).(func(string)); ok {
		fn(C.GoString(msg))
	}
}

//export goMenuCallback
func goMenuCallback(ptr unsafe.Pointer) {
	if fn, ok := getCallback(ptr).(func()); ok {
		fn()
	}
}
