// Package platform provides cross-platform abstractions for native GUI components.
//
// The platform layer abstracts:
//   - Application lifecycle (run loop, quit)
//   - Window management (create, show, resize)
//   - WebView (load content, execute JS, bidirectional messaging)
//   - Menus (app menu bar, context menus)
//
// Platform-specific implementations:
//   - darwin: Cocoa + WebKit (WKWebView with custom WebKit.framework)
//   - linux:  GTK + WebKitGTK (planned)
package platform

// Options configures the platform layer initialization.
type Options struct {
	// Name is the application name (shown in menus, dock, etc.)
	Name string

	// WebKitPath is the path to a custom WebKit.framework (optional).
	// If empty, uses the system WebKit.
	WebKitPath string

	// Debug enables developer tools and verbose logging.
	Debug bool
}

// Platform is the main entry point for platform services.
// Use New() to create a platform instance for the current OS.
type Platform interface {
	// Run starts the main event loop. Blocks until Quit() is called.
	Run() error

	// Quit exits the main event loop and terminates the application.
	Quit()

	// NewWindow creates a new native window.
	NewWindow(opts WindowOptions) (Window, error)

	// NewMenu creates a new menu that can be used as app menu or context menu.
	NewMenu() Menu

	// SetAppMenu sets the application menu bar.
	SetAppMenu(menu Menu)

	// Invoke runs a function on the main/UI thread.
	// Safe to call from any goroutine.
	Invoke(fn func())
}
