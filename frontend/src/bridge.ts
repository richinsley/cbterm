// Bridge to the Go backend (JS -> Go).
//
// The go-webui backend exposes its bound handler as a window global
// (window.cbtermToGo) once the WebSocket bridge has connected — which can
// happen AFTER this module has run, so early messages are queued until it
// appears. A legacy fallback to the old WKWebView message handler keeps the
// pre-migration build working.

const queue: string[] = [];
let timer: number | null = null;

function flush(): void {
    const fn = (window as any).cbtermToGo;
    if (typeof fn !== 'function') return;
    if (timer !== null) {
        clearInterval(timer);
        timer = null;
    }
    while (queue.length > 0) {
        fn(queue.shift()!);
    }
}

function send(msg: object | string): void {
    const s = typeof msg === 'string' ? msg : JSON.stringify(msg);

    const fn = (window as any).cbtermToGo;
    if (typeof fn === 'function') {
        fn(s);
        return;
    }

    // Legacy WKWebView backend
    const legacy = (window as any).webkit?.messageHandlers?.cbterm;
    if (legacy) {
        legacy.postMessage(s);
        return;
    }

    // go-webui bridge not connected yet — queue until it is
    queue.push(s);
    if (timer === null) {
        timer = window.setInterval(flush, 20);
    }
}

// Exposed to virtual API modules served by Go (e.g. the extension
// Terminal API stub), which can't import from the bundle.
(window as any).cbtermBridge = { send };

export default send;
