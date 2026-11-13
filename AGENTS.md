# AGENTS.md — cbterm

Project context for AI agents working on this codebase.

## Target

**cbterm** is an AI-native terminal emulator built with:
- **Go** for the backend and platform bindings
- **WebKit** for rendering (using xterm.js)
- **Custom protocol extensions** for AI integration (OSC 7776)

### Vision
- Single binary, no runtime dependencies
- Cross-platform: macOS first, Linux planned
- Structured AI interaction protocols, not bolted-on hacks
- Uses custom-built WebKit.framework for protocol extensions

---

## Work Completed

### 2026-02-02: Project Setup
- Created repo `github.com/calobozan/cbterm`
- WebKit CI build pipeline (GitHub Actions)
- Builds WebKit from Safari 17.5 tag, produces `webkit-macos-arm64.tar.gz`
- First successful release: `webkit-20260202-8a78584`

### 2026-02-03: Platform Layer
- **pkg/platform/darwin**: macOS implementation using CGo + Objective-C
  - `bridge.h` / `bridge.m`: C/ObjC bridge (~500 lines)
  - Window management via NSWindow
  - WebView via WKWebView (system WebKit for now)
  - Menu system via NSMenu (app menu + context menus)
  - JS→Go messaging via WKScriptMessageHandler
- Working demo app with test HTML page
- Message passing verified working

### 2026-02-03: MVP Terminal — COMPLETE ✅
- **frontend/**: xterm.js terminal UI
  - TypeScript + Vite build
  - WebGL renderer with canvas fallback
  - Native bridge via WKWebView messageHandlers (no WebSocket!)
  - Resize handling
- **pkg/terminal/terminal.go**: Clean PTY wrapper
  - Callback-based I/O (OnOutput func)
  - Base64 encoding for safe binary transport
  - Proper UTF-8 handling via Uint8Array
- **Native Bridge Architecture**
  - Input: JS `postMessage()` → Go → PTY
  - Output: PTY → Go → `EvalAsync()` → xterm.js
  - No WebSocket, no HTTP server for terminal I/O
  - Direct IPC through WKWebView
- **Fully functional terminal!**
  - Shell works, commands execute
  - TUI apps render correctly (tested with SSH + remote TUIs)
  - UTF-8/Unicode (Powerline fonts, Nerd Fonts) working
  - Window resize propagates to PTY

### 2026-02-03: Clipboard & Multi-Window
- **Clipboard support**
  - Cmd+C copies selected text
  - Cmd+V pastes from system clipboard
  - Cmd+A selects all
  - Uses browser Clipboard API via attachCustomKeyEventHandler
- **Multi-window support**
  - Cmd+N creates new terminal window
  - Each window has independent terminal session
  - App quits when last window closes
  - Refactored to App/TerminalWindow structure

### 2026-11-13: Control Read Path, Extension Output, Leak Fix
- **get_buffer works end-to-end**
  - SerializeAddon loaded on every terminal (frontend/src/tabs.ts)
  - `window.cbterm.getBuffer(id)` exposed; `App.GetTerminalBuffer` implemented
    (was a `return ''` TODO) — JSON-escaped terminal ID, no injection
  - Control mode `get_buffer` returns serialized terminal content
    (ANSI + scrollback) over the JSON protocol
- **Extension Terminal.onData wired**
  - `window.cbterm.write` fans output out to `_onDataCallbacks(data, terminalId)`
  - First working extension verified end-to-end: manifest discovery in Go →
    ES module load via `cbterm://extensions/` → `activate()` → onData chunks
- **Window leak fixed** (pkg/platform/darwin/bridge.m)
  - `windowShouldClose` releases the Go-owned window controller exactly once
    per closed window (balances the `__bridge_retained` +1)
  - Previously each closed window leaked its WKWebView + web content process
- **Smoke-tested via `--control`**: ping / get_state / type / get_buffer /
  quit all pass; marker string round-tripped PTY → xterm → serialize → JSON

---

## Work To Do

### Phase 1: Terminal Core ✅
- [x] Add xterm.js frontend (npm package)
- [x] PTY management (use `github.com/creack/pty`)
- [x] WebSocket bridge between PTY and xterm.js
- [x] Basic terminal working end-to-end

### Phase 2: Extension Foundation
- [x] JS extension loading mechanism
- [x] Extension ↔ Go message protocol
- [x] Enable more xterm.js addons (search, image, serialize)
- [x] Simple extension API

### Phase 3: Custom WebKit Integration
- [ ] Load custom WebKit.framework instead of system WebKit
- [ ] Implement OSC 7776 protocol extension in WebKit
- [ ] Bridge custom protocol to Go backend

### Phase 3: AI Protocol Extensions
- [ ] Define protocol spec (resurrect `protocol/SPEC.md`)
- [ ] Structured output regions
- [ ] AI context markers
- [ ] Tool invocation protocol

### Phase 4: Polish
- [ ] Proper app icon
- [ ] Preferences/settings
- [ ] Multiple tabs/windows
- [ ] Themes

### Phase 5: Linux
- [ ] `pkg/platform/linux` using GTK + WebKitGTK
- [ ] CI build for Linux

---

## Rules of Engagement

### Building
```bash
# Build (macOS only for now)
go build -o cbterm ./cmd/cbterm

# Run with debug mode (enables web inspector)
./cbterm --debug

# Run WebKit CI build
gh workflow run webkit-build.yml --repo calobozan/cbterm
```

### Code Style
- Go: standard `gofmt`, no external linters required
- ObjC: Cocoa conventions, ARC enabled
- Keep CGo boundaries clean — minimize back-and-forth

### Architecture Principles
1. **Platform abstraction**: All platform-specific code in `pkg/platform/<os>/`
2. **Message-passing**: JS↔Go communication via message handlers, not direct FFI
3. **Main thread safety**: UI operations must go through `Platform.Invoke()`
4. **No Electron**: We use native WebKit, not Chromium

### File Organization
```
cmd/cbterm/          # Main binary
pkg/platform/        # Platform abstraction interfaces
pkg/platform/darwin/ # macOS implementation
pkg/platform/linux/  # Linux implementation (planned)
pkg/terminal/        # Terminal/PTY logic (planned)
pkg/protocol/        # AI protocol extensions (planned)
frontend/            # xterm.js frontend (planned)
webkit/              # WebKit patches and build artifacts
```

### Testing
- Manual testing for now (GUI apps are hard to unit test)
- `./cbterm --debug` should open a window with working web inspector
- JS→Go messages should appear in stderr logs

### Commits
- Conventional commits preferred: `feat:`, `fix:`, `refactor:`, `docs:`
- Keep commits atomic and buildable

---

## Key Decisions

| Decision | Rationale |
|----------|-----------|
| Go + CGo | Rich's preferred language, good FFI story |
| WebKit over Chromium | Lighter weight, can patch for custom protocols |
| xterm.js | Battle-tested terminal renderer, easy to extend |
| No Electron/Tauri | Need custom WebKit, not system WebView |
| macOS first | Primary dev platform, Linux later via WebKitGTK |

---

## Resources

- [WebKit source](https://github.com/WebKit/WebKit)
- [xterm.js](https://xtermjs.org/)
- [OSC escape codes](https://invisible-island.net/xterm/ctlseqs/ctlseqs.html)
- [Kitty protocol extensions](https://sw.kovidgoyal.net/kitty/protocol-extensions/) (inspiration)

---

*Last updated: 2026-11-13 (control read path + extension output verified)*
