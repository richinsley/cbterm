# Term

A modern terminal emulator built with Go, WebKit, and xterm.js — designed from the ground up for AI integration.

## Vision

- **Single binary** — no runtime dependencies
- **AI-native** — structured protocols for AI interaction, not bolted-on hacks
- **Extensible** — custom terminal protocol extensions (inspired by Kitty)
- **Cross-platform** — macOS first, Linux and Windows planned

## Architecture

```
┌─────────────────────────────────────────┐
│           Native Window (WebKit)         │
│  ┌─────────────────────────────────────┐ │
│  │          xterm.js Frontend          │ │
│  └──────────────┬──────────────────────┘ │
└─────────────────┼───────────────────────┘
                  │ WebSocket
┌─────────────────┴───────────────────────┐
│            Go Backend                    │
│  ┌─────────────┐  ┌──────────────────┐  │
│  │   PTY Mgr   │  │  Protocol Ext    │  │
│  └─────────────┘  └──────────────────┘  │
└─────────────────────────────────────────┘
```

## Building

### Prerequisites

- Go 1.22+
- Node.js 20+ (for frontend build)
- macOS 14+ (for now)

### Quick Start

```bash
# Download prebuilt WebKit (from CI artifacts)
make webkit-download

# Build the terminal
make build

# Run
./bin/term
```

### Building WebKit from Source

WebKit is built via GitHub Actions and cached as release artifacts. To trigger a manual build:

```bash
gh workflow run webkit-build.yml
```

## Protocol Extensions

Term implements custom terminal protocol extensions for AI integration. See [protocol/SPEC.md](protocol/SPEC.md) for details.

## License

MIT
