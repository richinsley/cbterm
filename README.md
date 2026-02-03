# cbterm

A modern, extensible terminal emulator for macOS built with Go, WKWebView, and xterm.js.

![cbterm](https://img.shields.io/badge/version-0.0.1-blue) ![macOS](https://img.shields.io/badge/platform-macOS-lightgrey) ![License](https://img.shields.io/badge/license-MIT-green)

## Features

- **Tabs & Panes** — Split terminals vertically/horizontally, iTerm2-style UI
- **Native Performance** — WKWebView with WebGL-accelerated rendering
- **Extensions** — ES module-based extension system
- **Control Mode** — JSON API for automation and AI agents
- **Standalone Binary** — Single 4MB executable, no dependencies

## Installation

### Download

Download the latest release from [Releases](https://github.com/calobozan/cbterm/releases).

**macOS Security Note:** The binary is unsigned. On first run:
```bash
# Remove quarantine attribute
xattr -cr cbterm

# Or right-click → Open in Finder
```

### Build from Source

```bash
# Prerequisites: Go 1.21+, Node.js 18+

# Clone
git clone https://github.com/calobozan/cbterm.git
cd cbterm

# Build frontend
cd frontend && npm install && npm run build && cd ..

# Build binary
go build -o cbterm ./cmd/cbterm

# Run
./cbterm
```

## Usage

```bash
# Normal mode
./cbterm

# Debug mode (developer tools enabled)
./cbterm --debug

# Control mode (JSON API via stdin/stdout)
./cbterm --control
```

## Keyboard Shortcuts

| Shortcut | Action |
|----------|--------|
| `⌘T` | New tab |
| `⌘W` | Close pane |
| `⌘D` | Split vertically |
| `⇧⌘D` | Split horizontally |
| `⌘[` / `⌘]` | Previous/next tab |
| `⌘1-9` | Jump to tab |
| `⌥⌘←↑↓→` | Navigate panes |
| `⌘C` | Copy (with selection) |
| `⌘V` | Paste |
| `⌘A` | Select all |

## Extensions

Extensions live in `~/.config/cbterm/extensions/`:

```
~/.config/cbterm/extensions/
└── my-extension/
    ├── manifest.json
    └── index.js
```

### manifest.json

```json
{
  "name": "my-extension",
  "version": "1.0.0",
  "main": "index.js",
  "permissions": ["terminal.write"]
}
```

### index.js

```javascript
export function activate(context) {
  console.log('Extension activated!');
}

export function deactivate() {
  console.log('Extension deactivated');
}
```

See [docs/EXTENSIONS.md](docs/EXTENSIONS.md) for full documentation.

## Control Mode

Control mode enables programmatic interaction via JSON:

```bash
# Start in control mode
./cbterm --control

# Send commands
echo '{"cmd":"ping"}' 
# → {"type":"response","cmd":"ping","success":true,"data":{"pong":"cbterm"}}

echo '{"cmd":"type","terminal":"term-1","text":"ls -la\n"}'
# → {"type":"response","cmd":"type","success":true}
```

### Commands

| Command | Description |
|---------|-------------|
| `ping` | Health check |
| `get_state` | Get current windows/terminals |
| `new_tab` | Create new tab |
| `split` | Split active pane |
| `type` | Send keystrokes to terminal |
| `quit` | Exit application |

### Events

```json
{"type":"event","cmd":"terminal_created","data":{"terminal_id":"term-1"}}
{"type":"event","cmd":"terminal_output","data":{"terminal":"term-1","data":"<base64>"}}
```

See [docs/CONTROL.md](docs/CONTROL.md) for full protocol documentation.

## Architecture

```
cbterm
├── cmd/cbterm/          # Main application
├── pkg/
│   ├── platform/darwin/ # macOS Cocoa + WKWebView bridge
│   ├── terminal/        # PTY management
│   ├── control/         # JSON control interface
│   └── extensions/      # Extension discovery & loading
├── frontend/            # TypeScript + xterm.js
│   └── src/
│       ├── main.ts      # Entry point
│       └── tabs.ts      # Tab/pane management
└── assets.go            # Embedded frontend
```

### Custom URI Scheme

cbterm uses a custom `cbterm://` URI scheme to serve assets:

- `cbterm://app/` — Embedded frontend (from binary)
- `cbterm://extensions/` — User extensions
- `cbterm://themes/` — User themes
- `cbterm://api/` — Virtual API endpoints

This bypasses browser `file://` restrictions, enabling full ES module support.

## Development

```bash
# Watch frontend changes
cd frontend && npm run dev

# Run with debug mode
./cbterm --debug

# Right-click in terminal → Inspect Element
```

## License

MIT

## Credits

Built with:
- [xterm.js](https://xtermjs.org/) — Terminal rendering
- [creack/pty](https://github.com/creack/pty) — PTY management
- WebKit/WKWebView — Native web view
