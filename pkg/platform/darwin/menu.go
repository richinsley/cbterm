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

// Menu represents a macOS NSMenu.
type Menu struct {
	ptr C.CBTermMenu
}

func newMenu() *Menu {
	return &Menu{
		ptr: C.cbterm_menu_new(),
	}
}

// Add adds a menu item.
func (m *Menu) Add(item MenuItem) {
	if item.Label == "-" {
		m.AddSeparator()
		return
	}

	label := C.CString(item.Label)
	defer C.free(unsafe.Pointer(label))

	var shortcut *C.char
	if item.Shortcut != "" {
		shortcut = C.CString(item.Shortcut)
		defer C.free(unsafe.Pointer(shortcut))
	}

	var cb unsafe.Pointer
	if item.Action != nil {
		cb = registerCallback(item.Action)
	}

	// Handle submenu
	if item.Submenu != nil {
		C.cbterm_menu_add_submenu(m.ptr, label, item.Submenu.ptr)
		return
	}

	C.cbterm_menu_add_item(
		m.ptr,
		label,
		shortcut,
		C.bool(item.Enabled),
		C.bool(item.Checked),
		cb,
	)
}

// AddSeparator adds a separator line.
func (m *Menu) AddSeparator() {
	C.cbterm_menu_add_separator(m.ptr)
}

// Clear removes all items from the menu.
func (m *Menu) Clear() {
	C.cbterm_menu_clear(m.ptr)
}

// Popup shows this menu as a context menu.
func (m *Menu) Popup(x, y int) {
	C.cbterm_menu_popup(m.ptr, C.int(x), C.int(y))
}
