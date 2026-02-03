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
});

// ResizeObserver for container size changes
const resizeObserver = new ResizeObserver(() => {
    fitAddon.fit();
});
resizeObserver.observe(container);

// WebSocket connection to backend
let ws: WebSocket | null = null;
let reconnectAttempts = 0;
const maxReconnectAttempts = 5;

function connect() {
    // Get WebSocket URL from query params or default to same host
    const params = new URLSearchParams(window.location.search);
    const wsUrl = params.get('ws') || `ws://${window.location.host}/ws`;
    
    console.log(`Connecting to ${wsUrl}...`);
    ws = new WebSocket(wsUrl);
    
    ws.onopen = () => {
        console.log('WebSocket connected');
        reconnectAttempts = 0;
        
        // Send initial terminal size
        sendResize();
        
        // Notify native side if available
        postMessage({ type: 'connected' });
    };
    
    ws.onmessage = (event) => {
        // Write data from PTY to terminal
        term.write(event.data);
    };
    
    ws.onclose = () => {
        console.log('WebSocket disconnected');
        ws = null;
        
        // Attempt reconnect
        if (reconnectAttempts < maxReconnectAttempts) {
            reconnectAttempts++;
            const delay = Math.min(1000 * Math.pow(2, reconnectAttempts), 10000);
            console.log(`Reconnecting in ${delay}ms (attempt ${reconnectAttempts})...`);
            setTimeout(connect, delay);
        } else {
            term.write('\r\n\x1b[31mConnection lost. Reload to reconnect.\x1b[0m\r\n');
        }
    };
    
    ws.onerror = (error) => {
        console.error('WebSocket error:', error);
    };
}

// Send terminal size to backend
function sendResize() {
    if (ws && ws.readyState === WebSocket.OPEN) {
        const size = { cols: term.cols, rows: term.rows };
        ws.send(JSON.stringify({ type: 'resize', ...size }));
    }
}

// Handle terminal input
term.onData((data) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(data);
    }
});

// Handle terminal resize
term.onResize(({ cols, rows }) => {
    sendResize();
});

// Native message bridge (for communication with Go backend via WKWebView)
function postMessage(msg: object) {
    if ((window as any).webkit?.messageHandlers?.cbterm) {
        (window as any).webkit.messageHandlers.cbterm.postMessage(JSON.stringify(msg));
    }
}

// Expose terminal to global scope for debugging
(window as any).term = term;
(window as any).cbterm = {
    write: (data: string) => term.write(data),
    clear: () => term.clear(),
    reset: () => term.reset(),
    focus: () => term.focus(),
    fit: () => fitAddon.fit(),
    getSize: () => ({ cols: term.cols, rows: term.rows }),
};

// Start connection
connect();

// Focus terminal
term.focus();

console.log('cbterm initialized');
