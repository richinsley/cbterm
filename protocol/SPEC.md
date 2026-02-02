# Term Protocol Extensions

This document specifies the custom terminal protocol extensions implemented by Term.

## Overview

Term extends standard terminal escape sequences with custom extensions for:
- AI integration (structured context, annotations, queries)
- Rich graphics (inline images, charts, SVG)
- Structured data output (JSON, tables, trees)
- Interactive UI elements (buttons, links, inputs)

## Escape Sequence Format

Extensions use OSC (Operating System Command) sequences with a private-use code:

```
OSC 7776 ; <json-payload> BEL
or
OSC 7776 ; <json-payload> ST

Where:
- OSC = \x1b]  (Escape + ])
- 7776 = "TERM" on phone keypad
- BEL = \x07
- ST = \x1b\\ (String Terminator)
```

The payload is always JSON:

```json
{
  "type": "<extension-type>",
  "data": { ... }
}
```

## Extension Types

### AI Extensions (`ai.*`)

#### `ai.context.push`
Push context information for AI consumption. This data is not displayed but available to AI agents.

```json
{
  "type": "ai.context.push",
  "data": {
    "key": "working_directory",
    "value": "/home/user/project",
    "ttl": 60
  }
}
```

#### `ai.context.pop`
Remove a previously pushed context.

```json
{
  "type": "ai.context.pop",
  "data": {
    "key": "working_directory"
  }
}
```

#### `ai.boundary`
Mark command boundaries for AI to understand command start/end.

```json
{
  "type": "ai.boundary",
  "data": {
    "event": "start|end",
    "command": "npm install",
    "cwd": "/home/user/project",
    "exitCode": 0,
    "timestamp": 1699999999
  }
}
```

#### `ai.annotate`
AI can add annotations to terminal output (shown as overlays).

```json
{
  "type": "ai.annotate",
  "data": {
    "line": 42,
    "col": 10,
    "length": 25,
    "text": "This error indicates...",
    "style": "warning|error|info|hint"
  }
}
```

#### `ai.suggest`
Show inline AI suggestions.

```json
{
  "type": "ai.suggest",
  "data": {
    "text": "Did you mean: npm install --save-dev?",
    "action": "replace",
    "replacement": "npm install --save-dev"
  }
}
```

### Structured Data (`data.*`)

#### `data.json`
Emit structured JSON data alongside terminal output.

```json
{
  "type": "data.json",
  "data": {
    "channel": "default",
    "payload": { "files": [...], "count": 42 }
  }
}
```

#### `data.table`
Render a proper table (not ASCII art).

```json
{
  "type": "data.table",
  "data": {
    "headers": ["Name", "Size", "Modified"],
    "rows": [
      ["file.txt", "1.2KB", "2024-01-15"],
      ["image.png", "45KB", "2024-01-14"]
    ],
    "sortable": true
  }
}
```

#### `data.progress`
Rich progress indicators.

```json
{
  "type": "data.progress",
  "data": {
    "id": "download-1",
    "percent": 45,
    "text": "Downloading packages...",
    "style": "bar|spinner|dots"
  }
}
```

### Graphics (`gfx.*`)

#### `gfx.image`
Display inline images (inspired by Kitty graphics protocol).

```json
{
  "type": "gfx.image",
  "data": {
    "src": "base64:...",
    "format": "png|jpg|gif|webp",
    "width": 40,
    "height": 20,
    "unit": "cells|pixels",
    "alt": "Screenshot of app"
  }
}
```

#### `gfx.svg`
Inline SVG rendering.

```json
{
  "type": "gfx.svg",
  "data": {
    "content": "<svg>...</svg>",
    "width": 200,
    "height": 100
  }
}
```

#### `gfx.chart`
Data visualization.

```json
{
  "type": "gfx.chart",
  "data": {
    "type": "line|bar|pie",
    "data": {
      "labels": ["Jan", "Feb", "Mar"],
      "values": [10, 25, 15]
    },
    "width": 60,
    "height": 20
  }
}
```

### UI Elements (`ui.*`)

#### `ui.button`
Clickable buttons.

```json
{
  "type": "ui.button",
  "data": {
    "id": "submit-btn",
    "label": "Submit",
    "action": "emit",
    "payload": { "event": "submit" }
  }
}
```

#### `ui.link`
Enhanced hyperlinks with custom handlers.

```json
{
  "type": "ui.link",
  "data": {
    "url": "https://example.com",
    "text": "Click here",
    "handler": "external|internal|callback"
  }
}
```

### Navigation (`nav.*`)

#### `nav.fold`
Collapsible regions.

```json
{
  "type": "nav.fold",
  "data": {
    "event": "start|end",
    "id": "fold-1",
    "label": "Build output",
    "collapsed": true
  }
}
```

#### `nav.anchor`
Named anchors for jumping.

```json
{
  "type": "nav.anchor",
  "data": {
    "id": "error-section",
    "label": "First Error"
  }
}
```

## Detection

Programs can detect Term support by checking:

1. Environment variable: `TERM_PROTOCOL=1`
2. Query sequence: `OSC 7776 ; {"type":"query","data":{"capability":"*"}} BEL`

Response:
```json
{
  "type": "query.response",
  "data": {
    "version": "1.0",
    "capabilities": ["ai.*", "data.*", "gfx.*", "ui.*", "nav.*"]
  }
}
```

## Graceful Degradation

All extensions are designed to degrade gracefully in standard terminals:
- Unknown OSC sequences are typically ignored
- Programs should check for Term support before using extensions
- Fallback to standard terminal output when unsupported

## Shell Integration

For `ai.boundary` to work automatically, shell integration scripts are provided:

```bash
# In .bashrc/.zshrc
source /path/to/term/shell/integration.sh
```

This wraps PROMPT_COMMAND / precmd to emit boundary markers automatically.
