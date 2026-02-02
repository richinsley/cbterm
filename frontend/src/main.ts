import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { WebglAddon } from '@xterm/addon-webgl';
import { WebLinksAddon } from '@xterm/addon-web-links';
import '@xterm/xterm/css/xterm.css';

import { ProtocolHandler } from './extensions/protocol';

// Tokyo Night theme
const theme = {
  background: '#1a1b26',
  foreground: '#a9b1d6',
  cursor: '#c0caf5',
  cursorAccent: '#1a1b26',
  selectionBackground: '#33467c',
  black: '#15161e',
  red: '#f7768e',
  green: '#9ece6a',
  yellow: '#e0af68',
  blue: '#7aa2f7',
  magenta: '#bb9af7',
  cyan: '#7dcfff',
  white: '#a9b1d6',
  brightBlack: '#414868',
  brightRed: '#f7768e',
  brightGreen: '#9ece6a',
  brightYellow: '#e0af68',
  brightBlue: '#7aa2f7',
  brightMagenta: '#bb9af7',
  brightCyan: '#7dcfff',
  brightWhite: '#c0caf5',
};

class TermApp {
  private terminal: Terminal;
  private fitAddon: FitAddon;
  private ws: WebSocket | null = null;
  private protocol: ProtocolHandler;
  private reconnectTimeout: number | null = null;

  constructor() {
    this.terminal = new Terminal({
      theme,
      fontFamily: '"JetBrains Mono", "Fira Code", "SF Mono", Menlo, monospace',
      fontSize: 14,
      lineHeight: 1.2,
      cursorBlink: true,
      cursorStyle: 'bar',
      allowProposedApi: true,
    });

    this.fitAddon = new FitAddon();
    this.protocol = new ProtocolHandler(this.terminal);

    this.init();
  }

  private async init() {
    const container = document.getElementById('terminal')!;
    
    // Load addons
    this.terminal.loadAddon(this.fitAddon);
    this.terminal.loadAddon(new WebLinksAddon());
    
    // Open terminal
    this.terminal.open(container);
    
    // Try WebGL renderer for better performance
    try {
      this.terminal.loadAddon(new WebglAddon());
    } catch (e) {
      console.warn('WebGL addon failed to load, using canvas renderer');
    }

    // Fit to container
    this.fitAddon.fit();

    // Handle resize
    window.addEventListener('resize', () => this.handleResize());
    
    // Connect to backend
    this.connect();

    // Handle input
    this.terminal.onData((data) => {
      this.send(data);
    });

    // Focus terminal
    this.terminal.focus();
  }

  private connect() {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.host}/ws`;
    
    this.ws = new WebSocket(wsUrl);
    
    this.ws.onopen = () => {
      console.log('Connected to terminal backend');
      this.sendResize();
    };

    this.ws.onmessage = (event) => {
      if (typeof event.data === 'string') {
        // JSON protocol message
        try {
          const msg = JSON.parse(event.data);
          this.protocol.handle(msg);
        } catch (e) {
          // Plain text, write to terminal
          this.terminal.write(event.data);
        }
      } else if (event.data instanceof Blob) {
        // Binary terminal data
        event.data.arrayBuffer().then((buffer) => {
          this.terminal.write(new Uint8Array(buffer));
        });
      }
    };

    this.ws.onclose = () => {
      console.log('Disconnected from terminal backend');
      this.scheduleReconnect();
    };

    this.ws.onerror = (err) => {
      console.error('WebSocket error:', err);
    };
  }

  private scheduleReconnect() {
    if (this.reconnectTimeout) return;
    
    this.reconnectTimeout = window.setTimeout(() => {
      this.reconnectTimeout = null;
      this.connect();
    }, 2000);
  }

  private send(data: string | object) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      if (typeof data === 'string') {
        this.ws.send(data);
      } else {
        this.ws.send(JSON.stringify(data));
      }
    }
  }

  private handleResize() {
    this.fitAddon.fit();
    this.sendResize();
  }

  private sendResize() {
    this.send({
      type: 'resize',
      cols: this.terminal.cols,
      rows: this.terminal.rows,
    });
  }
}

// Start the app
new TermApp();
