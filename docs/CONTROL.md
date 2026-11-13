# cbterm Control Mode

Control mode enables programmatic interaction with cbterm via JSON commands on stdin/stdout. This is useful for:

- **AI Agents** — Let agents operate terminals programmatically
- **Testing** — Automated UI/integration tests
- **Scripting** — Automate terminal workflows

## Usage

```bash
cbterm --control
```

Send JSON commands (one per line) to stdin, receive JSON responses/events on stdout.

## Commands

Note: the `terminal` and `tab` fields may also be spelled `terminal_id` and
`tab_id` (the names used in responses and events); both are accepted.

### ping
Health check.

```json
{"cmd": "ping"}
```
Response:
```json
{"type": "response", "cmd": "ping", "success": true, "data": {"pong": "cbterm"}}
```

### get_state
Get current app state.

```json
{"cmd": "get_state"}
```
Response:
```json
{"type": "response", "cmd": "get_state", "success": true, "data": {"windows": 1, "terminals": ["term-1"]}}
```

### new_tab
Create a new tab with a terminal.

```json
{"cmd": "new_tab"}
```
Response:
```json
{"type": "response", "cmd": "new_tab", "success": true, "data": {"tab_id": "tab-2", "terminal_id": "term-2"}}
```

### close_tab
Close a tab.

```json
{"cmd": "close_tab", "tab": "tab-2"}
```

### split
Split the active pane.

```json
{"cmd": "split", "direction": "vertical"}
{"cmd": "split", "direction": "horizontal"}
```
Response:
```json
{"type": "response", "cmd": "split", "success": true, "data": {"terminal_id": "term-3"}}
```

### close_pane
Close a pane.

```json
{"cmd": "close_pane", "terminal": "term-3"}
```

### type
Send text to a terminal (like typing on keyboard).

```json
{"cmd": "type", "terminal": "term-1", "text": "echo hello\n"}
```

The `\n` sends Enter. Escape sequences work:
- `\n` — Enter
- `\t` — Tab
- `\x03` — Ctrl+C
- `\x04` — Ctrl+D

### get_buffer
Get the serialized content of a terminal (with ANSI escapes), as it
currently renders. The visible rows are always included.

```json
{"cmd": "get_buffer", "terminal": "term-1"}
```

Optionally add the last N lines of scrollback above the visible rows:

```json
{"cmd": "get_buffer", "terminal": "term-1", "scrollback": 50}
```

Response:
```json
{"type": "response", "cmd": "get_buffer", "success": true, "data": {"buffer": "..."}}
```

### eval_js
Run raw JavaScript in the app's UI. The script is executed as an async
function body, so a top-level `return` produces the result (returned as a
string — use `JSON.stringify` for structured values). Useful for
inspecting UI state (DOM, terminal dimensions, active tab) that the other
commands don't cover.

```json
{"cmd": "eval_js", "js": "return JSON.stringify({tabs: document.querySelectorAll('.tab').length})"}
```

Response:
```json
{"type": "response", "cmd": "eval_js", "success": true, "data": {"result": "{\"tabs\":2}"}}
```

Requires the single-client Script() path — not available with `--multi`.

### focus
Focus the cbterm window.

```json
{"cmd": "focus"}
```

### quit / exit / close
Close the application.

```json
{"cmd": "quit"}
```

## Events

Events are emitted automatically on stdout.

### terminal_created
```json
{"type": "event", "cmd": "terminal_created", "data": {"terminal_id": "term-1", "cols": 80, "rows": 24}}
```

### terminal_output
Terminal output (base64 encoded for binary safety).

```json
{"type": "event", "cmd": "terminal_output", "data": {"terminal": "term-1", "data": "SGVsbG8gV29ybGQK"}}
```

Decode with: `echo "SGVsbG8gV29ybGQK" | base64 -d`

### terminal_closed
```json
{"type": "event", "cmd": "terminal_closed", "data": {"terminal_id": "term-1"}}
```

## Request IDs

For correlating responses, include `request_id`:

```json
{"cmd": "type", "terminal": "term-1", "text": "ls\n", "request_id": "req-123"}
```
Response:
```json
{"type": "response", "request_id": "req-123", "cmd": "type", "success": true}
```

## Example: Agent Session

```bash
# Start cbterm in control mode
cbterm --control &

# Wait for terminal, then run commands
sleep 2
echo '{"cmd": "type", "terminal": "term-1", "text": "cd /tmp && ls\n"}'
sleep 1
echo '{"cmd": "type", "terminal": "term-1", "text": "echo done\n"}'
sleep 1
echo '{"cmd": "quit"}'
```

## Example: Python Agent

```python
import subprocess
import json
import base64

proc = subprocess.Popen(
    ['cbterm', '--control'],
    stdin=subprocess.PIPE,
    stdout=subprocess.PIPE,
    text=True
)

def send(cmd):
    proc.stdin.write(json.dumps(cmd) + '\n')
    proc.stdin.flush()

def recv():
    line = proc.stdout.readline()
    return json.loads(line) if line else None

# Wait for terminal
import time
time.sleep(2)

# Run a command
send({'cmd': 'type', 'terminal': 'term-1', 'text': 'uname -a\n'})

# Read output events
while True:
    event = recv()
    if event and event.get('cmd') == 'terminal_output':
        output = base64.b64decode(event['data']['data']).decode('utf-8', errors='replace')
        print(output, end='')
    # ... process events

send({'cmd': 'quit'})
```

## Tips

1. **Wait for terminal** — The terminal takes ~1-2s to initialize after launch
2. **Base64 output** — Terminal output is base64 encoded because it can contain binary escape sequences
3. **Newlines matter** — Include `\n` at the end of commands to "press Enter"
4. **Request IDs** — Use them if you need to match responses to requests in async code
