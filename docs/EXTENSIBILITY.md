# cbterm Extensibility Model

*Analysis of iTerm2 features and how cbterm should approach extensibility.*

## Philosophy

**Minimal core, maximum extensibility.**

cbterm should be a thin, fast, reliable terminal that users can extend to their needs. We don't want to ship bloat that 90% of users don't need.

## Extension Vectors

cbterm can be extended in three ways:

### 1. Go Extensions (Backend)
- Run in the Go process
- Full system access (files, network, etc.)
- Good for: shell integration, triggers, password managers, system integration
- Distribution: compiled into binary or dynamic plugins (future)

### 2. JavaScript Extensions (Frontend)
- Run in the WebKit context
- Access to xterm.js APIs and DOM
- Good for: UI features, text processing, decorations, themes
- Distribution: JS files loaded at startup

### 3. WASM Modules (Sandboxed)
- Run in WebAssembly sandbox
- Limited, capability-based access
- Good for: untrusted extensions, compute-heavy processing
- Distribution: .wasm files

## iTerm2 Feature Analysis

### Already in xterm.js (free)
| Feature | xterm.js Support |
|---------|------------------|
| 24-bit color | ✅ Built-in |
| Mouse reporting | ✅ Built-in |
| Unicode/emoji | ✅ Built-in |
| Selection | ✅ Built-in |
| WebGL rendering | ✅ addon-webgl (we use) |
| Link detection | ✅ addon-web-links (we use) |
| Auto-fit | ✅ addon-fit (we use) |
| Search | ✅ addon-search |
| Inline images | ✅ addon-image |
| Serialize/replay | ✅ addon-serialize |
| Ligatures | ✅ addon-ligatures |

### Core cbterm (should ship with)
| Feature | Rationale |
|---------|-----------|
| Single window/session | MVP baseline |
| PTY management | Core functionality |
| Native menus | Platform integration |
| Clipboard | Essential |
| Basic config | Themes, fonts, etc. |

### Extension Candidates

**JS Extensions (frontend):**
- Search UI (uses addon-search)
- Autocomplete popup
- Timestamps/annotations
- Badges/status display
- Smart selection enhancements
- Custom themes
- Keyboard shortcuts
- Command palette

**Go Extensions (backend):**
- Shell integration (prompt detection, cwd tracking)
- Triggers (regex → action)
- Password manager (keychain integration)
- Session restore
- Notifications (system integration)
- Hotkey window
- SSH config integration
- Multiple tabs/panes (window management)

**WASM Extensions:**
- Syntax highlighting for `cat` output
- Log parsing/filtering
- Custom escape sequence handlers
- AI integrations (sandboxed)

## Extension API Design (Draft)

### JS Extension Interface
```typescript
interface CBTermExtension {
  name: string;
  version: string;
  
  // Lifecycle
  activate(context: ExtensionContext): void;
  deactivate(): void;
}

interface ExtensionContext {
  terminal: Terminal;           // xterm.js instance
  subscriptions: Disposable[];  // cleanup tracking
  
  // APIs
  registerCommand(id: string, handler: () => void): void;
  registerKeybinding(key: string, command: string): void;
  showQuickPick(items: string[]): Promise<string>;
  showInputBox(options: InputBoxOptions): Promise<string>;
  
  // Communication with Go
  sendMessage(type: string, data: any): void;
  onMessage(type: string, handler: (data: any) => void): void;
}
```

### Go Extension Interface
```go
type Extension interface {
    Name() string
    Version() string
    
    // Lifecycle
    Activate(ctx ExtensionContext) error
    Deactivate() error
}

type ExtensionContext interface {
    // Terminal I/O
    OnOutput(fn func(data []byte))
    WriteInput(data []byte)
    
    // PTY info
    GetSize() (rows, cols int)
    GetCwd() string
    GetEnv() map[string]string
    
    // Communication with JS
    SendToFrontend(msgType string, data interface{})
    OnFrontendMessage(msgType string, fn func(data json.RawMessage))
    
    // System
    ShowNotification(title, body string)
    GetFromKeychain(key string) (string, error)
}
```

## What NOT to Build Into Core

- Git integration (extension)
- Docker integration (extension)
- Kubernetes integration (extension)
- AI features (extension, via protocol)
- Syntax highlighting (extension)
- Session/pane management beyond single window (extension)
- Anything cloud-connected (extension)

## Implementation Phases

### Phase 1: MVP ✅
- Single terminal window
- PTY + xterm.js + native bridge
- Basic menus

### Phase 2: Extension Foundation
- JS extension loading
- Extension ↔ Go message passing
- Simple extension API

### Phase 3: More xterm.js Addons
- Enable search, image, serialize addons
- Expose via extension API

### Phase 4: Go Extension System
- Plugin architecture
- Shell integration extension
- Trigger extension

### Phase 5: WASM Support
- WASM runtime in frontend
- Capability-based security
- Example extensions

## Open Questions

1. **Extension distribution**: Package manager? Git repos? Local files only?
2. **Security model**: How to sandbox JS extensions? Trust levels?
3. **Configuration**: TOML? YAML? Lua? JSON?
4. **Hot reload**: Can extensions reload without restarting terminal?

---

*This is a living document. Update as design evolves.*
