package platform

// WindowOptions configures a new window.
type WindowOptions struct {
	// Title is the window title.
	Title string

	// Width and Height are the initial window dimensions.
	Width  int
	Height int

	// X and Y are the initial window position (optional).
	// If both are 0, the window is centered on screen.
	X int
	Y int

	// Resizable controls whether the window can be resized.
	Resizable bool

	// Frameless removes the window title bar and frame.
	Frameless bool

	// AlwaysOnTop keeps the window above other windows.
	AlwaysOnTop bool
}

// Window represents a native window.
type Window interface {
	// Show makes the window visible.
	Show()

	// Hide makes the window invisible.
	Hide()

	// Close closes and destroys the window.
	Close()

	// Focus brings the window to the front and gives it focus.
	Focus()

	// Center centers the window on the screen.
	Center()

	// SetTitle sets the window title.
	SetTitle(title string)

	// SetSize sets the window dimensions.
	SetSize(width, height int)

	// SetPosition sets the window position.
	SetPosition(x, y int)

	// SetMinSize sets the minimum window dimensions.
	SetMinSize(width, height int)

	// SetMaxSize sets the maximum window dimensions.
	SetMaxSize(width, height int)

	// GetWebView returns the window's web view.
	GetWebView() WebView

	// OnClose registers a callback for when the window is about to close.
	// Return false from the callback to prevent the window from closing.
	OnClose(fn func() bool)
}
