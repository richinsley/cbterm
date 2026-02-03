import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { WebglAddon } from '@xterm/addon-webgl';
import { WebLinksAddon } from '@xterm/addon-web-links';
import '@xterm/xterm/css/xterm.css';

// Terminal configuration
const term = new Terminal({
    cursorBlink: true,
    cursorStyle: 'block',
    fontSize: 14,
    fontFamily: '"SF Mono", Menlo, Monaco, "Courier New", monospace',
    theme: {
        background: '#1e1e1e',
        foreground: '#d4d4d4',
        cursor: '#d4d4d4',
        cursorAccent: '#1e1e1e',
        selectionBackground: '#264f78',
        black: '#000000',
        red: '#cd3131',
        green: '#0dbc79',
        yellow: '#e5e510',
        blue: '#2472c8',
        magenta: '#bc3fbc',
        cyan: '#11a8cd',
        white: '#e5e5e5',
        brightBlack: '#666666',
        brightRed: '#f14c4c',
        brightGreen: '#23d18b',
        brightYellow: '#f5f543',
        brightBlue: '#3b8eea',
        brightMagenta: '#d670d6',
        brightCyan: '#29b8db',
        brightWhite: '#ffffff',
    },
    allowProposedApi: true,
});

// Addons
const fitAddon = new FitAddon();
term.loadAddon(fitAddon);
term.loadAddon(new WebLinksAddon());

// Mount terminal
const container = document.getElementById('terminal')!;
term.open(container);

// Try WebGL renderer, fall back to canvas
try {
    const webglAddon = new WebglAddon();
    webglAddon.onContextLoss(() => {
        webglAddon.dispose();
    });
    term.loadAddon(webglAddon);
    console.log('WebGL renderer enabled');
} catch (e) {
    console.warn('WebGL not available, using canvas renderer');
}

// Fit terminal to container
fitAddon.fit();

// Handle window resize
window.addEventListener('resize', () => {
    fitAddon.fit();
    sendResize();
});

// ResizeObserver for container size changes
const resizeObserver = new ResizeObserver(() => {
    fitAddon.fit();
    sendResize();
});
resizeObserver.observe(container);

// Native bridge - communicate directly with Go via WKWebView
const bridge = (window as any).webkit?.messageHandlers?.cbterm;

function postMessage(msg: object | string) {
    if (bridge) {
        const data = typeof msg === 'string' ? msg : JSON.stringify(msg);
        bridge.postMessage(data);
    }
}

// Send terminal size to backend
function sendResize() {
    postMessage({ type: 'resize', cols: term.cols, rows: term.rows });
}

// Handle terminal input - send to Go
term.onData((data) => {
    // Base64 encode for safe transport
    const encoded = btoa(data);
    postMessage({ type: 'input', data: encoded });
});

// Handle terminal resize
term.onResize(({ cols, rows }) => {
    postMessage({ type: 'resize', cols, rows });
});

// API exposed to Go for writing output
(window as any).cbterm = {
    // Write base64-encoded data to terminal
    write: (base64Data: string) => {
        try {
            // Decode base64 to binary string, then to Uint8Array
            // This preserves UTF-8 byte sequences correctly
            const binary = atob(base64Data);
            const bytes = new Uint8Array(binary.length);
            for (let i = 0; i < binary.length; i++) {
                bytes[i] = binary.charCodeAt(i);
            }
            term.write(bytes);
        } catch (e) {
            console.error('Failed to decode terminal data:', e);
        }
    },
    
    // Direct write (for simple ASCII)
    writeRaw: (data: string) => {
        term.write(data);
    },
    
    // Terminal control
    clear: () => term.clear(),
    reset: () => term.reset(),
    focus: () => term.focus(),
    fit: () => {
        fitAddon.fit();
        sendResize();
    },
    
    // Get terminal dimensions
    getSize: () => ({ cols: term.cols, rows: term.rows }),
};

// Signal ready to Go
postMessage({ type: 'ready', cols: term.cols, rows: term.rows });

// Focus terminal
term.focus();

console.log('cbterm initialized (native bridge)');
