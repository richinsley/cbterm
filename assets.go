// Package cbterm provides embedded assets.
package cbterm

import "embed"

// FrontendAssets contains the built frontend files (index.html, JS, CSS).
// Build with: cd frontend && npm run build
//
//go:embed frontend/dist/*
var FrontendAssets embed.FS
