package platform

// MenuItem represents a single menu item.
type MenuItem struct {
	// Label is the menu item text. Use "-" for a separator.
	Label string

	// Shortcut is the keyboard shortcut (e.g., "Cmd+Q", "Ctrl+C").
	// Platform-specific: Cmd on macOS, Ctrl on others.
	Shortcut string

	// Enabled controls whether the item is clickable.
	Enabled bool

	// Checked shows a checkmark next to the item.
	Checked bool

	// Action is called when the item is clicked.
	Action func()

	// Submenu is a nested menu (optional).
	Submenu Menu
}

// Menu represents a menu (app menu bar, submenu, or context menu).
type Menu interface {
	// Add adds a menu item.
	Add(item MenuItem)

	// AddSeparator adds a separator line.
	AddSeparator()

	// Clear removes all items from the menu.
	Clear()

	// Popup shows this menu as a context menu at the given position.
	// If x and y are both -1, shows at the current mouse position.
	Popup(x, y int)
}

// StandardMenuItems provides platform-appropriate standard menu items.
type StandardMenuItems struct {
	// App menu (macOS only)
	About     MenuItem
	Preferences MenuItem
	Quit      MenuItem

	// Edit menu
	Undo      MenuItem
	Redo      MenuItem
	Cut       MenuItem
	Copy      MenuItem
	Paste     MenuItem
	SelectAll MenuItem

	// Window menu
	Minimize  MenuItem
	Zoom      MenuItem
	Close     MenuItem
}

// GetStandardMenuItems returns standard menu items with platform-appropriate
// labels and shortcuts. Actions must be set by the caller.
func GetStandardMenuItems(appName string) StandardMenuItems {
	return StandardMenuItems{
		About:       MenuItem{Label: "About " + appName},
		Preferences: MenuItem{Label: "Settings...", Shortcut: "Cmd+,"},
		Quit:        MenuItem{Label: "Quit " + appName, Shortcut: "Cmd+Q"},

		Undo:      MenuItem{Label: "Undo", Shortcut: "Cmd+Z"},
		Redo:      MenuItem{Label: "Redo", Shortcut: "Cmd+Shift+Z"},
		Cut:       MenuItem{Label: "Cut", Shortcut: "Cmd+X"},
		Copy:      MenuItem{Label: "Copy", Shortcut: "Cmd+C"},
		Paste:     MenuItem{Label: "Paste", Shortcut: "Cmd+V"},
		SelectAll: MenuItem{Label: "Select All", Shortcut: "Cmd+A"},

		Minimize: MenuItem{Label: "Minimize", Shortcut: "Cmd+M"},
		Zoom:     MenuItem{Label: "Zoom"},
		Close:    MenuItem{Label: "Close Window", Shortcut: "Cmd+W"},
	}
}
