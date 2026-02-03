// Tab and pane management for cbterm

import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { WebglAddon } from '@xterm/addon-webgl';
import { WebLinksAddon } from '@xterm/addon-web-links';
import { SearchAddon } from '@xterm/addon-search';
import { ImageAddon } from '@xterm/addon-image';

// Types
export interface Tab {
    id: string;
    title: string;
    rootPane: Pane;
}

export interface Pane {
    id: string;
    type: 'terminal' | 'split';
    // Terminal panes
    terminal?: TerminalInstance;
    // Split panes
    direction?: 'horizontal' | 'vertical';
    children?: Pane[];
    sizes?: number[];
    // DOM
    element?: HTMLElement;
}

export interface TerminalInstance {
    id: string;
    terminal: Terminal;
    fitAddon: FitAddon;
    searchAddon: SearchAddon;
    element: HTMLElement;
}

// Message handler type
type MessageHandler = (msg: object) => void;

// Tab Manager
export class TabManager {
    private tabs: Map<string, Tab> = new Map();
    private activeTabId: string | null = null;
    private tabCounter = 0;
    private terminalCounter = 0;
    private paneCounter = 0;
    
    private tabBar: HTMLElement;
    private contentArea: HTMLElement;
    private postMessage: MessageHandler;
    
    // Track active pane for keyboard focus
    private activePaneId: string | null = null;
    
    constructor(container: HTMLElement, postMessage: MessageHandler) {
        this.postMessage = postMessage;
        
        this.tabBar = document.createElement('div');
        this.tabBar.className = 'tab-bar';
        // Initialize as hidden since we start with 0 tabs before createTab is called
        this.tabBar.classList.add('hidden'); 
        container.appendChild(this.tabBar);
        
        this.contentArea = document.createElement('div');
        this.contentArea.className = 'tab-content';
        container.appendChild(this.contentArea);
        
        this.renderTabBar();
        document.addEventListener('keydown', this.handleKeyDown.bind(this));
    }
    
    // Helper method to manage visibility
    private updateTabBarVisibility(): void {
        if (this.tabs.size > 1) {
            this.tabBar.classList.remove('hidden');
        } else {
            this.tabBar.classList.add('hidden');
        }
        // Trigger a resize to ensure xterm.js fills the newly available space
        this.handleResize();
    }

    // Create a new tab with a single terminal pane
    createTab(title?: string): Tab {
        const tabId = `tab-${++this.tabCounter}`;
        const pane = this.createTerminalPane();
        
        const tab: Tab = {
            id: tabId,
            title: title || `Terminal ${this.tabCounter}`,
            rootPane: pane,
        };
        
        this.tabs.set(tabId, tab);
        this.renderTabBar();
        this.activateTab(tabId);
        
        // Update visibility after adding
        this.updateTabBarVisibility();
        
        return tab;
    }
    
    // Create a terminal pane
    private createTerminalPane(): Pane {
        const paneId = `pane-${++this.paneCounter}`;
        const terminalId = `term-${++this.terminalCounter}`;
        
        // Create terminal element with titlebar
        const element = document.createElement('div');
        element.className = 'terminal-pane';
        element.dataset.paneId = paneId;
        
        // Create titlebar
        const titlebar = document.createElement('div');
        titlebar.className = 'pane-titlebar';
        
        // Left side - close button
        const leftSide = document.createElement('div');
        leftSide.className = 'pane-titlebar-left';
        
        const closeBtn = document.createElement('div');
        closeBtn.className = 'pane-btn pane-btn-close';
        closeBtn.innerHTML = '×';
        closeBtn.title = 'Close pane';
        closeBtn.addEventListener('click', (e) => {
            e.stopPropagation();
            this.closePane(paneId);
        });
        leftSide.appendChild(closeBtn);
        
        // Center - title (will be updated with shell info)
        const center = document.createElement('div');
        center.className = 'pane-titlebar-center';
        center.textContent = 'zsh'; // Default, could be updated based on process
        
        // Right side - menu button
        const rightSide = document.createElement('div');
        rightSide.className = 'pane-titlebar-right';
        
        const menuBtn = document.createElement('div');
        menuBtn.className = 'pane-btn pane-btn-menu';
        menuBtn.innerHTML = '⋯';
        menuBtn.title = 'Pane options';
        menuBtn.addEventListener('click', (e) => {
            e.stopPropagation();
            this.showPaneContextMenu(paneId, e.clientX, e.clientY);
        });
        rightSide.appendChild(menuBtn);
        
        titlebar.appendChild(leftSide);
        titlebar.appendChild(center);
        titlebar.appendChild(rightSide);
        element.appendChild(titlebar);
        
        // Create terminal container
        const terminalContainer = document.createElement('div');
        terminalContainer.className = 'pane-terminal';
        element.appendChild(terminalContainer);
        
        // Create xterm instance
        const terminal = new Terminal({
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
        
        const fitAddon = new FitAddon();
        const searchAddon = new SearchAddon();
        const imageAddon = new ImageAddon();
        
        terminal.loadAddon(fitAddon);
        terminal.loadAddon(searchAddon);
        terminal.loadAddon(imageAddon);
        terminal.loadAddon(new WebLinksAddon());
        
        const terminalInstance: TerminalInstance = {
            id: terminalId,
            terminal,
            fitAddon,
            searchAddon,
            element: terminalContainer, // Terminal mounts in the container, not the pane
        };
        
        const pane: Pane = {
            id: paneId,
            type: 'terminal',
            terminal: terminalInstance,
            element,
        };
        
        // Wire up terminal events
        terminal.onData((data) => {
            const encoded = btoa(data);
            this.postMessage({ type: 'input', terminalId, data: encoded });
        });
        
        terminal.onResize(({ cols, rows }) => {
            this.postMessage({ type: 'resize', terminalId, cols, rows });
        });
        
        // Handle keyboard shortcuts (Cmd+C, Cmd+V, etc.)
        terminal.attachCustomKeyEventHandler((event) => {
            // Cmd+C (copy) - only if there's a selection
            if (event.metaKey && event.key === 'c' && terminal.hasSelection()) {
                if (event.type === 'keydown') {
                    const selection = terminal.getSelection();
                    if (selection) {
                        navigator.clipboard.writeText(selection);
                    }
                }
                return false; // Prevent default and don't send to PTY
            }
            
            // Cmd+V (paste)
            if (event.metaKey && event.key === 'v') {
                if (event.type === 'keydown') {
                    navigator.clipboard.readText().then((text) => {
                        if (text) {
                            const encoded = btoa(text);
                            this.postMessage({ type: 'input', terminalId, data: encoded });
                        }
                    });
                }
                return false; // Prevent default and don't send to PTY
            }
            
            // Cmd+A (select all)
            if (event.metaKey && event.key === 'a') {
                if (event.type === 'keydown') {
                    terminal.selectAll();
                }
                return false;
            }
            
            // Let other keys through to terminal
            return true;
        });
        
        // Focus handling - click anywhere on pane to focus
        element.addEventListener('mousedown', () => {
            this.setActivePane(paneId);
        });
        
        // Also focus on titlebar click
        titlebar.addEventListener('mousedown', () => {
            this.setActivePane(paneId);
        });
        
        return pane;
    }
    
    // Split the active pane
    splitPane(direction: 'horizontal' | 'vertical'): Pane | null {
        if (!this.activePaneId || !this.activeTabId) return null;
        
        const tab = this.tabs.get(this.activeTabId);
        if (!tab) return null;
        
        const targetPane = this.findPane(tab.rootPane, this.activePaneId);
        if (!targetPane || targetPane.type !== 'terminal') return null;
        
        // Create new terminal pane
        const newPane = this.createTerminalPane();
        
        // Convert target pane to split pane
        const parentPane = this.findParentPane(tab.rootPane, this.activePaneId);
        
        if (parentPane && parentPane.type === 'split' && parentPane.direction === direction) {
            // Add to existing split
            parentPane.children!.push(newPane);
            parentPane.sizes = parentPane.children!.map(() => 100 / parentPane.children!.length);
        } else {
            // Create new split
            const splitPane: Pane = {
                id: `pane-${++this.paneCounter}`,
                type: 'split',
                direction,
                children: [{ ...targetPane }, newPane],
                sizes: [50, 50],
            };
            
            // Replace target with split
            if (tab.rootPane.id === targetPane.id) {
                tab.rootPane = splitPane;
            } else if (parentPane) {
                const idx = parentPane.children!.findIndex(p => p.id === targetPane.id);
                if (idx !== -1) {
                    parentPane.children![idx] = splitPane;
                }
            }
        }
        
        this.renderTab(tab);
        this.setActivePane(newPane.id);
        
        // Request new terminal from backend
        this.postMessage({ 
            type: 'new_terminal', 
            terminalId: newPane.terminal!.id,
            cols: newPane.terminal!.terminal.cols,
            rows: newPane.terminal!.terminal.rows,
        });
        
        return newPane;
    }
    
    // Close a pane
    closePane(paneId: string): void {
        if (!this.activeTabId) return;
        
        const tab = this.tabs.get(this.activeTabId);
        if (!tab) return;
        
        // If it's the only pane, close the tab
        if (tab.rootPane.id === paneId) {
            this.closeTab(tab.id);
            return;
        }
        
        const parent = this.findParentPane(tab.rootPane, paneId);
        if (!parent || parent.type !== 'split') return;
        
        const idx = parent.children!.findIndex(p => p.id === paneId);
        if (idx === -1) return;
        
        // Remove pane
        const closedPane = parent.children!.splice(idx, 1)[0];
        if (closedPane.terminal) {
            closedPane.terminal.terminal.dispose();
            this.postMessage({ type: 'close_terminal', terminalId: closedPane.terminal.id });
        }
        
        // If only one child left, replace parent with child
        if (parent.children!.length === 1) {
            const remaining = parent.children![0];
            Object.assign(parent, remaining);
            delete parent.children;
            delete parent.sizes;
        } else {
            // Recalculate sizes
            parent.sizes = parent.children!.map(() => 100 / parent.children!.length);
        }
        
        this.renderTab(tab);
        
        // Activate another pane
        const firstTerminal = this.findFirstTerminalPane(tab.rootPane);
        if (firstTerminal) {
            this.setActivePane(firstTerminal.id);
        }
    }
    
    // Activate a tab
    activateTab(tabId: string): void {
        const tab = this.tabs.get(tabId);
        if (!tab) return;
        
        this.activeTabId = tabId;
        this.renderTabBar();
        this.renderTab(tab);
        
        // Activate first terminal pane
        const firstTerminal = this.findFirstTerminalPane(tab.rootPane);
        if (firstTerminal) {
            this.setActivePane(firstTerminal.id);
        }
    }
    
    // Close a tab
    closeTab(tabId: string): void {
        const tab = this.tabs.get(tabId);
        if (!tab) return;
        
        this.disposePane(tab.rootPane);
        this.tabs.delete(tabId);
        
        if (this.activeTabId === tabId) {
            const remaining = Array.from(this.tabs.keys());
            if (remaining.length > 0) {
                this.activateTab(remaining[remaining.length - 1]);
            } else {
                this.createTab();
            }
        }
        
        this.renderTabBar();
        // Update visibility after removing
        this.updateTabBarVisibility();
    }
    
    // Set active pane (for focus)
    setActivePane(paneId: string): void {
        this.activePaneId = paneId;
        
        // Update visual focus - remove from all panes
        document.querySelectorAll('.terminal-pane').forEach(el => {
            el.classList.remove('focused');
        });
        
        // Add focus to the target pane
        const paneEl = document.querySelector(`.terminal-pane[data-pane-id="${paneId}"]`);
        if (paneEl) {
            paneEl.classList.add('focused');
        }
        
        // Focus/blur terminals
        if (!this.activeTabId) return;
        const tab = this.tabs.get(this.activeTabId);
        if (!tab) return;
        
        // Blur all terminals in this tab, then focus the active one
        this.blurAllTerminals(tab.rootPane);
        
        const pane = this.findPane(tab.rootPane, paneId);
        if (pane?.terminal) {
            pane.terminal.terminal.focus();
        }
    }
    
    // Blur all terminals in a pane tree
    private blurAllTerminals(pane: Pane): void {
        if (pane.terminal) {
            pane.terminal.terminal.blur();
        }
        if (pane.children) {
            for (const child of pane.children) {
                this.blurAllTerminals(child);
            }
        }
    }
    
    // Write data to a terminal
    writeToTerminal(terminalId: string, data: string): void {
        for (const tab of this.tabs.values()) {
            const pane = this.findPaneByTerminalId(tab.rootPane, terminalId);
            if (pane?.terminal) {
                try {
                    const binary = atob(data);
                    const bytes = new Uint8Array(binary.length);
                    for (let i = 0; i < binary.length; i++) {
                        bytes[i] = binary.charCodeAt(i);
                    }
                    pane.terminal.terminal.write(bytes);
                } catch (e) {
                    console.error('Failed to decode terminal data:', e);
                }
                return;
            }
        }
    }
    
    // Find a pane by ID
    private findPane(root: Pane, paneId: string): Pane | null {
        if (root.id === paneId) return root;
        if (root.children) {
            for (const child of root.children) {
                const found = this.findPane(child, paneId);
                if (found) return found;
            }
        }
        return null;
    }
    
    // Find pane by terminal ID
    private findPaneByTerminalId(root: Pane, terminalId: string): Pane | null {
        if (root.terminal?.id === terminalId) return root;
        if (root.children) {
            for (const child of root.children) {
                const found = this.findPaneByTerminalId(child, terminalId);
                if (found) return found;
            }
        }
        return null;
    }
    
    // Find parent of a pane
    private findParentPane(root: Pane, paneId: string): Pane | null {
        if (root.children) {
            for (const child of root.children) {
                if (child.id === paneId) return root;
                const found = this.findParentPane(child, paneId);
                if (found) return found;
            }
        }
        return null;
    }
    
    // Find first terminal pane
    private findFirstTerminalPane(root: Pane): Pane | null {
        if (root.type === 'terminal') return root;
        if (root.children) {
            for (const child of root.children) {
                const found = this.findFirstTerminalPane(child);
                if (found) return found;
            }
        }
        return null;
    }
    
    // Dispose all terminals in a pane tree
    private disposePane(pane: Pane): void {
        if (pane.terminal) {
            pane.terminal.terminal.dispose();
            this.postMessage({ type: 'close_terminal', terminalId: pane.terminal.id });
        }
        if (pane.children) {
            for (const child of pane.children) {
                this.disposePane(child);
            }
        }
    }
    
    // Render the tab bar
    private renderTabBar(): void {
        this.tabBar.innerHTML = '';
        
        let tabIndex = 0;
        for (const tab of this.tabs.values()) {
            tabIndex++;
            const tabEl = document.createElement('div');
            tabEl.className = 'tab' + (tab.id === this.activeTabId ? ' active' : '');
            tabEl.dataset.tabId = tab.id;
            
            const titleEl = document.createElement('span');
            titleEl.className = 'tab-title';
            // Show tab number for first 9 tabs (Cmd+1-9 shortcut)
            const shortcut = tabIndex <= 9 ? `⌘${tabIndex} ` : '';
            titleEl.textContent = shortcut + tab.title;
            tabEl.appendChild(titleEl);
            
            const closeEl = document.createElement('span');
            closeEl.className = 'tab-close';
            closeEl.textContent = '×';
            closeEl.addEventListener('click', (e) => {
                e.stopPropagation();
                this.closeTab(tab.id);
            });
            tabEl.appendChild(closeEl);
            
            tabEl.addEventListener('click', () => {
                this.activateTab(tab.id);
            });
            
            this.tabBar.appendChild(tabEl);
        }
        
        // New tab button
        const newTabBtn = document.createElement('div');
        newTabBtn.className = 'tab new-tab';
        newTabBtn.textContent = '+';
        newTabBtn.addEventListener('click', () => {
            this.createTab();
            // Request new terminal
            const tab = this.tabs.get(this.activeTabId!);
            if (tab) {
                const pane = this.findFirstTerminalPane(tab.rootPane);
                if (pane?.terminal) {
                    this.postMessage({
                        type: 'new_terminal',
                        terminalId: pane.terminal.id,
                        cols: pane.terminal.terminal.cols,
                        rows: pane.terminal.terminal.rows,
                    });
                }
            }
        });
        this.tabBar.appendChild(newTabBtn);
    }
    
    // Render a tab's content
    private renderTab(tab: Tab): void {
        this.contentArea.innerHTML = '';
        const paneEl = this.renderPane(tab.rootPane);
        this.contentArea.appendChild(paneEl);
        
        // Fit all terminals
        this.fitAllTerminals(tab.rootPane);
    }
    
    // Render a pane tree
    private renderPane(pane: Pane): HTMLElement {
        if (pane.type === 'terminal') {
            // pane.element is the outer .terminal-pane div (with titlebar)
            // pane.terminal.element is the inner .pane-terminal div (where xterm mounts)
            const paneEl = pane.element!;
            const terminalContainer = pane.terminal!.element;
            
            // Mount terminal if not already
            if (!terminalContainer.querySelector('.xterm')) {
                console.log('Mounting terminal:', pane.terminal!.id);
                pane.terminal!.terminal.open(terminalContainer);
                console.log('Terminal mounted, size:', pane.terminal!.terminal.cols, 'x', pane.terminal!.terminal.rows);
                this.postMessage({ type: 'terminal_mounted', terminalId: pane.terminal!.id });
                
                // Try WebGL
                try {
                    const webglAddon = new WebglAddon();
                    webglAddon.onContextLoss(() => webglAddon.dispose());
                    pane.terminal!.terminal.loadAddon(webglAddon);
                    console.log('WebGL addon loaded');
                } catch (e) {
                    console.log('WebGL failed, using canvas');
                }
            }
            
            return paneEl;
        }
        
        // Split pane
        const container = document.createElement('div');
        container.className = `split-pane ${pane.direction}`;
        
        pane.children!.forEach((child, i) => {
            const childEl = this.renderPane(child);
            childEl.style.flexBasis = `${pane.sizes![i]}%`;
            container.appendChild(childEl);
            
            // Add divider (except after last)
            if (i < pane.children!.length - 1) {
                const divider = document.createElement('div');
                divider.className = 'pane-divider';
                divider.addEventListener('mousedown', (e) => {
                    this.startResize(e, pane, i);
                });
                container.appendChild(divider);
            }
        });
        
        return container;
    }
    
    // Fit all terminals in a pane tree
    private fitAllTerminals(pane: Pane): void {
        if (pane.terminal) {
            setTimeout(() => {
                pane.terminal!.fitAddon.fit();
                this.postMessage({
                    type: 'resize',
                    terminalId: pane.terminal!.id,
                    cols: pane.terminal!.terminal.cols,
                    rows: pane.terminal!.terminal.rows,
                });
            }, 50);
        }
        if (pane.children) {
            for (const child of pane.children) {
                this.fitAllTerminals(child);
            }
        }
    }
    
    // Handle pane divider resize
    private startResize(e: MouseEvent, pane: Pane, dividerIndex: number): void {
        e.preventDefault();
        
        const isHorizontal = pane.direction === 'horizontal';
        const startPos = isHorizontal ? e.clientX : e.clientY;
        const startSizes = [...pane.sizes!];
        
        const onMouseMove = (moveE: MouseEvent) => {
            const currentPos = isHorizontal ? moveE.clientX : moveE.clientY;
            const delta = currentPos - startPos;
            const containerSize = isHorizontal 
                ? this.contentArea.clientWidth 
                : this.contentArea.clientHeight;
            const deltaPercent = (delta / containerSize) * 100;
            
            const newSizes = [...startSizes];
            newSizes[dividerIndex] = Math.max(10, startSizes[dividerIndex] + deltaPercent);
            newSizes[dividerIndex + 1] = Math.max(10, startSizes[dividerIndex + 1] - deltaPercent);
            
            pane.sizes = newSizes;
            
            // Update DOM
            const container = this.contentArea.querySelector(`.split-pane.${pane.direction}`);
            if (container) {
                const children = container.children;
                let childIdx = 0;
                for (let i = 0; i < children.length; i++) {
                    const child = children[i] as HTMLElement;
                    if (!child.classList.contains('pane-divider')) {
                        child.style.flexBasis = `${pane.sizes[childIdx]}%`;
                        childIdx++;
                    }
                }
            }
            
            // Refit terminals
            this.fitAllTerminals(pane);
        };
        
        const onMouseUp = () => {
            document.removeEventListener('mousemove', onMouseMove);
            document.removeEventListener('mouseup', onMouseUp);
        };
        
        document.addEventListener('mousemove', onMouseMove);
        document.addEventListener('mouseup', onMouseUp);
    }
    
    // Handle keyboard shortcuts
    private handleKeyDown(e: KeyboardEvent): void {
        // Cmd+T: New tab
        if (e.metaKey && e.key === 't') {
            e.preventDefault();
            const tab = this.createTab();
            const pane = this.findFirstTerminalPane(tab.rootPane);
            if (pane?.terminal) {
                this.postMessage({
                    type: 'new_terminal',
                    terminalId: pane.terminal.id,
                    cols: pane.terminal.terminal.cols,
                    rows: pane.terminal.terminal.rows,
                });
            }
        }
        
        // Cmd+W: Close pane/tab
        if (e.metaKey && e.key === 'w') {
            e.preventDefault();
            if (this.activePaneId) {
                this.closePane(this.activePaneId);
            }
        }
        
        // Cmd+D: Split vertically
        if (e.metaKey && e.key === 'd') {
            e.preventDefault();
            this.splitPane('vertical');
        }
        
        // Cmd+Shift+D: Split horizontally
        if (e.metaKey && e.shiftKey && e.key === 'd') {
            e.preventDefault();
            this.splitPane('horizontal');
        }
        
        // Cmd+[/]: Navigate tabs
        if (e.metaKey && e.key === '[') {
            e.preventDefault();
            this.navigateTab(-1);
        }
        if (e.metaKey && e.key === ']') {
            e.preventDefault();
            this.navigateTab(1);
        }
        
        // Cmd+Option+Arrow: Navigate panes
        if (e.metaKey && e.altKey) {
            if (['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown'].includes(e.key)) {
                e.preventDefault();
                this.navigatePane(e.key.replace('Arrow', '').toLowerCase() as any);
            }
        }
        
        // Cmd+1-9: Switch to tab by number
        if (e.metaKey && !e.shiftKey && !e.altKey) {
            const num = parseInt(e.key);
            if (num >= 1 && num <= 9) {
                e.preventDefault();
                const tabIds = Array.from(this.tabs.keys());
                if (num <= tabIds.length) {
                    this.activateTab(tabIds[num - 1]);
                }
            }
        }
    }
    
    // Navigate between tabs
    navigateTab(delta: number): void {
        const tabIds = Array.from(this.tabs.keys());
        if (tabIds.length < 2 || !this.activeTabId) return;
        
        const currentIdx = tabIds.indexOf(this.activeTabId);
        const newIdx = (currentIdx + delta + tabIds.length) % tabIds.length;
        this.activateTab(tabIds[newIdx]);
    }
    
    // Close the active pane
    closeActivePane(): void {
        if (this.activePaneId) {
            this.closePane(this.activePaneId);
        }
    }
    
    // Close pane by terminal ID (called when shell exits)
    closePaneByTerminal(terminalId: string): void {
        for (const tab of this.tabs.values()) {
            const pane = this.findPaneByTerminalId(tab.rootPane, terminalId);
            if (pane) {
                this.closePane(pane.id);
                return;
            }
        }
    }
    
    // Navigate between panes
    private navigatePane(direction: 'left' | 'right' | 'up' | 'down'): void {
        if (!this.activeTabId || !this.activePaneId) return;
        
        const tab = this.tabs.get(this.activeTabId);
        if (!tab) return;
        
        // Get all terminal panes in order
        const terminalPanes: Pane[] = [];
        this.collectTerminalPanes(tab.rootPane, terminalPanes);
        
        if (terminalPanes.length < 2) return;
        
        const currentIndex = terminalPanes.findIndex(p => p.id === this.activePaneId);
        if (currentIndex === -1) return;
        
        // For simple left/right navigation, just go to prev/next
        // For up/down, we'd need to track spatial layout (TODO: proper 2D navigation)
        let newIndex: number;
        
        switch (direction) {
            case 'left':
            case 'up':
                newIndex = (currentIndex - 1 + terminalPanes.length) % terminalPanes.length;
                break;
            case 'right':
            case 'down':
                newIndex = (currentIndex + 1) % terminalPanes.length;
                break;
        }
        
        this.setActivePane(terminalPanes[newIndex].id);
    }
    
    // Collect all terminal panes from a tree (depth-first)
    private collectTerminalPanes(pane: Pane, result: Pane[]): void {
        if (pane.type === 'terminal') {
            result.push(pane);
        } else if (pane.children) {
            for (const child of pane.children) {
                this.collectTerminalPanes(child, result);
            }
        }
    }
    
    // Show context menu for a pane
    private showPaneContextMenu(paneId: string, x: number, y: number): void {
        // Remove any existing menu
        this.hideContextMenu();
        
        const menu = document.createElement('div');
        menu.className = 'pane-context-menu';
        
        const items = [
            { label: 'Split Vertically', action: () => { this.setActivePane(paneId); this.splitPane('vertical'); } },
            { label: 'Split Horizontally', action: () => { this.setActivePane(paneId); this.splitPane('horizontal'); } },
            { separator: true },
            { label: 'Clear Buffer', action: () => this.clearPaneBuffer(paneId) },
            { separator: true },
            { label: 'Close Pane', action: () => this.closePane(paneId) },
        ];
        
        for (const item of items) {
            if (item.separator) {
                const sep = document.createElement('div');
                sep.className = 'pane-context-menu-separator';
                menu.appendChild(sep);
            } else {
                const menuItem = document.createElement('div');
                menuItem.className = 'pane-context-menu-item';
                menuItem.textContent = item.label!;
                menuItem.addEventListener('click', () => {
                    item.action!();
                    this.hideContextMenu();
                });
                menu.appendChild(menuItem);
            }
        }
        
        // Add to DOM
        document.body.appendChild(menu);
        
        // Position menu, clamping to viewport
        const menuRect = menu.getBoundingClientRect();
        const viewportWidth = window.innerWidth;
        const viewportHeight = window.innerHeight;
        
        let menuX = x;
        let menuY = y;
        
        // Clamp to right edge
        if (menuX + menuRect.width > viewportWidth) {
            menuX = viewportWidth - menuRect.width - 8;
        }
        // Clamp to bottom edge
        if (menuY + menuRect.height > viewportHeight) {
            menuY = viewportHeight - menuRect.height - 8;
        }
        // Clamp to left/top edges
        menuX = Math.max(8, menuX);
        menuY = Math.max(8, menuY);
        
        menu.style.left = `${menuX}px`;
        menu.style.top = `${menuY}px`;
        
        // Close on click outside
        const closeHandler = (e: MouseEvent) => {
            if (!menu.contains(e.target as Node)) {
                this.hideContextMenu();
                document.removeEventListener('click', closeHandler);
            }
        };
        setTimeout(() => document.addEventListener('click', closeHandler), 0);
        
        // Close on escape
        const escHandler = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                this.hideContextMenu();
                document.removeEventListener('keydown', escHandler);
            }
        };
        document.addEventListener('keydown', escHandler);
    }
    
    // Hide context menu
    private hideContextMenu(): void {
        const existing = document.querySelector('.pane-context-menu');
        if (existing) {
            existing.remove();
        }
    }
    
    // Clear a pane's terminal buffer
    private clearPaneBuffer(paneId: string): void {
        if (!this.activeTabId) return;
        const tab = this.tabs.get(this.activeTabId);
        if (!tab) return;
        
        const pane = this.findPane(tab.rootPane, paneId);
        if (pane?.terminal) {
            pane.terminal.terminal.clear();
        }
    }
    
    // Get the active terminal ID (for initial ready message)
    getActiveTerminalId(): string | null {
        if (!this.activeTabId) return null;
        const tab = this.tabs.get(this.activeTabId);
        if (!tab) return null;
        const pane = this.findPane(tab.rootPane, this.activePaneId!);
        return pane?.terminal?.id || null;
    }
    
    // Resize handler
    handleResize(): void {
        if (!this.activeTabId) return;
        const tab = this.tabs.get(this.activeTabId);
        if (tab) {
            this.fitAllTerminals(tab.rootPane);
        }
    }
}
