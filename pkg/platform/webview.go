package platform

// WebViewOptions configures a new web view.
type WebViewOptions struct {
	// DevTools enables the web inspector / developer tools.
	DevTools bool

	// Transparent makes the web view background transparent.
	Transparent bool

	// UserAgent sets a custom user agent string.
	UserAgent string
}

// WebView represents a web view capable of rendering HTML/JS content.
type WebView interface {
	// LoadURL loads a URL into the web view.
	LoadURL(url string)

	// LoadHTML loads HTML content directly into the web view.
	// BaseURL is used to resolve relative URLs (optional).
	LoadHTML(html string, baseURL string)

	// Eval executes JavaScript in the web view and returns the result.
	// The result is JSON-encoded if it's an object/array.
	Eval(js string) (string, error)

	// Bind exposes a Go function to JavaScript.
	// The function can be called from JS as: window.<name>(...args)
	// Arguments and return values are JSON-encoded.
	Bind(name string, fn interface{}) error

	// OnMessage registers a handler for messages sent from JavaScript.
	// JS sends messages via: window.webkit.messageHandlers.cbterm.postMessage(msg)
	OnMessage(fn func(msg string))

	// OnNavigate registers a handler called when navigation occurs.
	// Return false to cancel the navigation.
	OnNavigate(fn func(url string) bool)

	// Reload reloads the current page.
	Reload()

	// SetDevToolsEnabled enables or disables developer tools.
	SetDevToolsEnabled(enabled bool)
}

// MessageHandler is the signature for JavaScript message handlers.
type MessageHandler func(msg string)
