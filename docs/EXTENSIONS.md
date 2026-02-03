# cbterm Extension Architecture

cbterm uses a custom URI scheme (`cbterm://`) to load all frontend assets. This bypasses browser security restrictions on `file://` URLs and enables a clean, secure extension system.

## URI Scheme

```
cbterm://app/           → Core frontend (bundled with app)
cbterm://extensions/    → User-installed extensions
cbterm://themes/        → Custom themes/color schemes
cbterm://api/           → Virtual API endpoints (future)
```

## Why Custom Schemes?

Browser security policies heavily restrict `file://` URLs:
- ❌ No ES module imports (`import` fails with CORS errors)
- ❌ No `fetch()` to local files
- ❌ No Web Workers
- ❌ Inconsistent localStorage behavior

With `cbterm://`:
- ✅ Full ES module support
- ✅ `fetch()` works for any `cbterm://` URL
- ✅ Web Workers load correctly
- ✅ Consistent, predictable behavior
- ✅ Security controlled by Go, not browser policies

## Extension Structure

```
~/.config/cbterm/extensions/
└── my-extension/
    ├── manifest.json      # Extension metadata
    ├── index.js           # Main entry point (ES module)
    ├── styles.css         # Optional styles
    └── assets/            # Images, fonts, etc.
```

### manifest.json

```json
{
  "name": "my-extension",
  "version": "1.0.0",
  "description": "A cool extension",
  "main": "index.js",
  "permissions": ["terminal.read", "terminal.write", "keybindings"],
  "activationEvents": ["onStartup"]
}
```

### index.js

```javascript
// Loaded as ES module - full import/export support
import { Terminal } from 'cbterm://api/terminal';

export function activate(context) {
  // Register commands, keybindings, etc.
  context.registerCommand('my-extension.hello', () => {
    console.log('Hello from extension!');
  });
  
  // Hook into terminal events
  Terminal.onData((data) => {
    // Process terminal output
  });
}

export function deactivate() {
  // Cleanup
}
```

## Loading Extensions

Extensions are discovered and loaded at startup:

1. **Discovery**: Scan `~/.config/cbterm/extensions/` for `manifest.json` files
2. **Validation**: Check manifest, verify permissions
3. **Registration**: Register URI paths for each extension
4. **Loading**: Frontend requests `cbterm://extensions/{name}/index.js`
5. **Activation**: Call `activate()` based on `activationEvents`

## Security Model

### Path Sandboxing
Each extension can only access its own directory. Requests like:
```
cbterm://extensions/evil-ext/../other-ext/secrets.json
```
Are blocked by path traversal detection in Go.

### Permission System
Extensions declare required permissions in `manifest.json`. Sensitive APIs require explicit grants:

| Permission | Description |
|------------|-------------|
| `terminal.read` | Read terminal output |
| `terminal.write` | Send input to terminal |
| `filesystem.read` | Read files (with user prompt) |
| `filesystem.write` | Write files (with user prompt) |
| `network` | Make HTTP requests |
| `keybindings` | Register keyboard shortcuts |
| `clipboard` | Access clipboard |

### Content Security
- Extensions run in the same WebView but isolated via ES modules
- No `eval()` or inline scripts in extension HTML
- CSP headers enforced for extension content

## Theme System

Themes are simpler than extensions — just CSS variables:

```
~/.config/cbterm/themes/
└── dracula/
    ├── manifest.json
    └── theme.css
```

### theme.css
```css
:root {
  --term-background: #282a36;
  --term-foreground: #f8f8f2;
  --term-cursor: #f8f8f2;
  --term-black: #000000;
  --term-red: #ff5555;
  /* ... */
}
```

## API Surface (Future)

```javascript
// Available at cbterm://api/*

// Terminal API
import { Terminal } from 'cbterm://api/terminal';
Terminal.write(data);
Terminal.onData(callback);
Terminal.resize(cols, rows);

// Config API
import { Config } from 'cbterm://api/config';
const theme = await Config.get('theme');
await Config.set('fontSize', 14);

// Keybindings API
import { Keybindings } from 'cbterm://api/keybindings';
Keybindings.register('Ctrl+Shift+P', 'commandPalette.open');

// Commands API
import { Commands } from 'cbterm://api/commands';
Commands.register('myext.doThing', handler);
Commands.execute('myext.doThing');
```

## Implementation Status

- [x] Custom URI scheme handler (`cbterm://`)
- [x] Path traversal protection
- [x] MIME type detection
- [ ] Extension discovery
- [ ] Extension manifest validation
- [ ] Permission system
- [ ] Frontend extension loader
- [ ] Extension API surface
- [ ] Theme system
- [ ] Extension packaging/distribution
