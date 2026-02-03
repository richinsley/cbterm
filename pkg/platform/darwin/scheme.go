//go:build darwin

package darwin

/*
#include "bridge.h"
#include <stdlib.h>
#include <string.h>
*/
import "C"

import (
	"path/filepath"
	"strings"
	"sync"
	"unsafe"
)

// SchemeHandler is called when the webview requests a URL with a custom scheme.
// Return the content bytes and MIME type, or nil/empty to return 404.
type SchemeHandler func(url string) (data []byte, mimeType string)

var (
	schemeHandlers   = make(map[string]SchemeHandler)
	schemeHandlersMu sync.RWMutex
)

// RegisterScheme registers a custom URI scheme handler.
// Must be called BEFORE creating any windows.
//
// Example:
//
//	darwin.RegisterScheme("cbterm", func(url string) ([]byte, string) {
//	    // url = "cbterm://extensions/foo/index.js"
//	    path := strings.TrimPrefix(url, "cbterm://")
//	    data, _ := os.ReadFile(filepath.Join(extensionsDir, path))
//	    return data, getMimeType(path)
//	})
func RegisterScheme(scheme string, handler SchemeHandler) {
	schemeHandlersMu.Lock()
	schemeHandlers[scheme] = handler
	schemeHandlersMu.Unlock()

	s := C.CString(scheme)
	defer C.free(unsafe.Pointer(s))

	C.cbterm_register_scheme(s)
}

// handleSchemeRequest is called from Objective-C when a custom scheme URL is requested
func handleSchemeRequest(url string) ([]byte, string) {
	// Extract scheme from URL
	scheme := ""
	if idx := strings.Index(url, "://"); idx != -1 {
		scheme = url[:idx]
	}

	schemeHandlersMu.RLock()
	handler, ok := schemeHandlers[scheme]
	schemeHandlersMu.RUnlock()

	if !ok || handler == nil {
		return nil, ""
	}

	return handler(url)
}

//export goSchemeCallback
func goSchemeCallback(urlC *C.char, dataOut **C.char, lengthOut *C.int, mimeTypeOut **C.char) {
	url := C.GoString(urlC)

	data, mimeType := handleSchemeRequest(url)

	if data == nil || len(data) == 0 {
		*dataOut = nil
		*lengthOut = 0
		return
	}

	// Allocate memory for the response (will be freed by Obj-C side)
	*dataOut = (*C.char)(C.malloc(C.size_t(len(data))))
	C.memcpy(unsafe.Pointer(*dataOut), unsafe.Pointer(&data[0]), C.size_t(len(data)))
	*lengthOut = C.int(len(data))

	if mimeType != "" {
		*mimeTypeOut = C.CString(mimeType)
	}
}

// Common MIME types for convenience
var mimeTypes = map[string]string{
	".html":  "text/html",
	".htm":   "text/html",
	".css":   "text/css",
	".js":    "application/javascript",
	".mjs":   "application/javascript",
	".json":  "application/json",
	".png":   "image/png",
	".jpg":   "image/jpeg",
	".jpeg":  "image/jpeg",
	".gif":   "image/gif",
	".svg":   "image/svg+xml",
	".ico":   "image/x-icon",
	".woff":  "font/woff",
	".woff2": "font/woff2",
	".ttf":   "font/ttf",
	".otf":   "font/otf",
	".wasm":  "application/wasm",
	".txt":   "text/plain",
	".md":    "text/markdown",
}

// MimeTypeForPath returns the MIME type for a file path based on extension.
func MimeTypeForPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if mime, ok := mimeTypes[ext]; ok {
		return mime
	}
	return "application/octet-stream"
}
