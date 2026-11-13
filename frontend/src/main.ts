import '@xterm/xterm/css/xterm.css';
import './tabs.css';
import { TabManager } from './tabs';

// Native bridge
const bridge = (window as any).webkit?.messageHandlers?.cbterm;

function postMessage(msg: object | string) {
    if (bridge) {
        const data = typeof msg === 'string' ? msg : JSON.stringify(msg);
        bridge.postMessage(data);
    }
}

// Initialize container
const container = document.getElementById('terminal')!;
container.style.display = 'flex';
container.style.flexDirection = 'column';
container.style.height = '100%';

// Create tab manager
const tabManager = new TabManager(container, postMessage);

// Create initial tab
const initialTab = tabManager.createTab();

// Handle resize
window.addEventListener('resize', () => {
    tabManager.handleResize();
});

const resizeObserver = new ResizeObserver(() => {
    tabManager.handleResize();
});
resizeObserver.observe(container);

// API exposed to Go for writing output and control
(window as any).cbterm = {
    // Write base64-encoded data to terminal, then notify extension subscribers
    write: (terminalId: string, base64Data: string) => {
        tabManager.writeToTerminal(terminalId, base64Data);
        const callbacks = (window as any).cbterm._onDataCallbacks as ((data: string, terminalId: string) => void)[];
        for (const cb of callbacks) {
            try {
                cb(base64Data, terminalId);
            } catch (e) {
                console.error('onData callback failed:', e);
            }
        }
    },
    
    // Legacy single-terminal write (for backwards compat during transition)
    writeLegacy: (base64Data: string) => {
        // Write to the first/active terminal
        const activeId = tabManager.getActiveTerminalId();
        if (activeId) {
            tabManager.writeToTerminal(activeId, base64Data);
        }
    },
    
    // Tab/pane management
    createTab: () => tabManager.createTab(),
    closeTab: (tabId: string) => tabManager.closeTab(tabId),
    splitHorizontal: () => tabManager.splitPane('horizontal'),
    splitVertical: () => tabManager.splitPane('vertical'),
    closePane: (paneId: string) => tabManager.closePane(paneId),
    closeActivePane: () => tabManager.closeActivePane(),
    closePaneByTerminal: (terminalId: string) => tabManager.closePaneByTerminal(terminalId),
    prevTab: () => tabManager.navigateTab(-1),
    nextTab: () => tabManager.navigateTab(1),
    
    // State inspection
    getActiveTerminalId: () => tabManager.getActiveTerminalId(),
    getBuffer: (terminalId: string) => tabManager.getBuffer(terminalId),
    
    // Extension support
    // Callbacks are invoked with (base64Data, terminalId) for every
    // terminal output chunk, after it has been written to the terminal.
    _onDataCallbacks: [] as ((data: string, terminalId: string) => void)[],
};

// Extension system
interface ExtensionManifest {
    name: string;
    version: string;
    description: string;
    main: string;
    permissions: string[];
    activationEvents: string[];
}

interface Extension {
    manifest: ExtensionManifest;
    path: string;
    enabled: boolean;
    error?: string;
}

interface ExtensionModule {
    activate?: (context: ExtensionContext) => void | Promise<void>;
    deactivate?: () => void | Promise<void>;
}

interface ExtensionContext {
    extensionPath: string;
    subscriptions: { dispose: () => void }[];
}

const loadedExtensions: Map<string, { module: ExtensionModule; context: ExtensionContext }> = new Map();

async function loadExtensions() {
    try {
        const response = await fetch('cbterm://api/extensions/list.json');
        if (!response.ok) return;
        
        const extensions: Extension[] = await response.json();
        
        for (const ext of extensions) {
            try {
                const moduleUrl = `cbterm://extensions/${ext.manifest.name}/${ext.manifest.main}`;
                const module: ExtensionModule = await import(/* @vite-ignore */ moduleUrl);
                
                const context: ExtensionContext = {
                    extensionPath: `cbterm://extensions/${ext.manifest.name}`,
                    subscriptions: [],
                };
                
                if (module.activate) {
                    await module.activate(context);
                }
                
                loadedExtensions.set(ext.manifest.name, { module, context });
                postMessage({ type: 'extension_loaded', name: ext.manifest.name, version: ext.manifest.version });
            } catch (e) {
                postMessage({ type: 'extension_error', name: ext.manifest.name, error: String(e) });
            }
        }
    } catch (e) {
        console.error('Failed to load extensions:', e);
    }
}

async function unloadExtensions() {
    for (const [name, { module, context }] of loadedExtensions) {
        try {
            if (module.deactivate) {
                await module.deactivate();
            }
            for (const sub of context.subscriptions) {
                sub.dispose();
            }
        } catch (e) {
            console.error(`Failed to deactivate extension ${name}:`, e);
        }
    }
    loadedExtensions.clear();
}

window.addEventListener('beforeunload', () => {
    unloadExtensions();
});

// Load extensions
loadExtensions().then(() => {
    postMessage({ type: 'extensions_loaded', count: loadedExtensions.size });
}).catch((e) => {
    postMessage({ type: 'extensions_error', error: String(e) });
});

// Get initial terminal info and signal ready
const firstPane = initialTab.rootPane;
if (firstPane.terminal) {
    // Wait a bit for terminal to be mounted and sized
    setTimeout(() => {
        const term = firstPane.terminal!;
        postMessage({ 
            type: 'ready', 
            terminalId: term.id,
            cols: term.terminal.cols, 
            rows: term.terminal.rows 
        });
        term.terminal.focus();
    }, 100);
}

console.log('cbterm initialized with tabs and panes');
