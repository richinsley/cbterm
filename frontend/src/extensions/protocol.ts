import { Terminal } from '@xterm/xterm';

export interface ProtocolMessage {
  type: string;
  data: any;
}

export class ProtocolHandler {
  private terminal: Terminal;
  private annotations: Map<string, HTMLElement> = new Map();

  constructor(terminal: Terminal) {
    this.terminal = terminal;
  }

  handle(msg: ProtocolMessage) {
    const [category, action] = msg.type.split('.');
    
    switch (category) {
      case 'ai':
        this.handleAI(action, msg.data);
        break;
      case 'data':
        this.handleData(action, msg.data);
        break;
      case 'gfx':
        this.handleGraphics(action, msg.data);
        break;
      case 'ui':
        this.handleUI(action, msg.data);
        break;
      case 'nav':
        this.handleNav(action, msg.data);
        break;
      default:
        console.log('Unknown protocol message:', msg);
    }
  }

  private handleAI(action: string, data: any) {
    switch (action) {
      case 'annotate':
        this.addAnnotation(data);
        break;
      case 'suggest':
        this.showSuggestion(data);
        break;
      case 'boundary':
        // Track command boundaries - useful for semantic scrollback
        console.log('Command boundary:', data);
        break;
      default:
        console.log('Unknown AI action:', action, data);
    }
  }

  private handleData(action: string, data: any) {
    switch (action) {
      case 'table':
        this.renderTable(data);
        break;
      case 'progress':
        this.updateProgress(data);
        break;
      case 'json':
        // Structured data - could be used by AI or displayed
        console.log('Structured data:', data.channel, data.payload);
        break;
      default:
        console.log('Unknown data action:', action, data);
    }
  }

  private handleGraphics(action: string, data: any) {
    switch (action) {
      case 'image':
        this.renderImage(data);
        break;
      case 'svg':
        this.renderSVG(data);
        break;
      case 'chart':
        this.renderChart(data);
        break;
      default:
        console.log('Unknown graphics action:', action, data);
    }
  }

  private handleUI(action: string, data: any) {
    switch (action) {
      case 'button':
        this.renderButton(data);
        break;
      case 'link':
        // Enhanced links handled by web-links addon or custom
        break;
      default:
        console.log('Unknown UI action:', action, data);
    }
  }

  private handleNav(action: string, data: any) {
    switch (action) {
      case 'fold':
        this.handleFold(data);
        break;
      case 'anchor':
        // Store anchor for navigation
        console.log('Anchor:', data);
        break;
      default:
        console.log('Unknown nav action:', action, data);
    }
  }

  // AI Extensions

  private addAnnotation(data: {
    line: number;
    col: number;
    length: number;
    text: string;
    style: string;
  }) {
    // TODO: Render annotation overlay at the specified position
    // This requires hooking into xterm.js rendering
    console.log('Annotation:', data);
  }

  private showSuggestion(data: {
    text: string;
    action: string;
    replacement?: string;
  }) {
    // TODO: Show inline suggestion UI
    console.log('Suggestion:', data);
  }

  // Data Extensions

  private renderTable(data: {
    headers: string[];
    rows: string[][];
    sortable?: boolean;
  }) {
    // For now, render as formatted text
    // TODO: Rich table rendering with click-to-sort
    const colWidths = data.headers.map((h, i) => {
      const maxContent = Math.max(h.length, ...data.rows.map(r => (r[i] || '').length));
      return Math.min(maxContent + 2, 40);
    });

    // Header
    let output = '\r\n';
    output += data.headers.map((h, i) => h.padEnd(colWidths[i])).join('│') + '\r\n';
    output += colWidths.map(w => '─'.repeat(w)).join('┼') + '\r\n';
    
    // Rows
    for (const row of data.rows) {
      output += row.map((c, i) => (c || '').padEnd(colWidths[i])).join('│') + '\r\n';
    }
    
    this.terminal.write(output);
  }

  private updateProgress(data: {
    id: string;
    percent: number;
    text: string;
    style: string;
  }) {
    // TODO: Rich progress bar rendering
    console.log('Progress:', data);
  }

  // Graphics Extensions

  private renderImage(data: {
    src: string;
    format: string;
    width: number;
    height: number;
    unit: string;
    alt: string;
  }) {
    // TODO: Render inline image using canvas or img overlay
    // xterm.js has experimental image support we could leverage
    console.log('Image:', data.alt || 'image', data.width, 'x', data.height);
  }

  private renderSVG(data: { content: string; width: number; height: number }) {
    // TODO: Render SVG overlay
    console.log('SVG:', data.width, 'x', data.height);
  }

  private renderChart(data: {
    type: string;
    data: { labels: string[]; values: number[] };
    width: number;
    height: number;
  }) {
    // TODO: Render chart using Chart.js or similar
    console.log('Chart:', data.type, data.data.labels);
  }

  // UI Extensions

  private renderButton(data: {
    id: string;
    label: string;
    action: string;
    payload: any;
  }) {
    // TODO: Render clickable button in terminal
    // For now, just show as text
    this.terminal.write(`\r\n[${data.label}]\r\n`);
  }

  // Nav Extensions

  private handleFold(data: {
    event: string;
    id: string;
    label: string;
    collapsed: boolean;
  }) {
    // TODO: Implement collapsible regions
    console.log('Fold:', data.event, data.label);
  }
}
