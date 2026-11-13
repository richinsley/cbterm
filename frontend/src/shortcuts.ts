// Keyboard shortcuts — stand-in for the native macOS menus, which the
// go-webui backend doesn't have. Cmd+T/W/D/[/1-9 are already handled by
// TabManager (tabs.ts); Cmd+C/V/A by xterm's custom key handler. This
// covers the two remaining old menu items: New Window and Reload.

import send from './bridge';

export function installShortcuts(): void {
    window.addEventListener('keydown', (e: KeyboardEvent) => {
        if (!e.metaKey || e.ctrlKey || e.altKey) return;

        switch (e.key) {
            case 'n':
            case 'N':
                e.preventDefault();
                send({ type: 'new_window' });
                break;
            case 'r':
            case 'R':
                e.preventDefault();
                location.reload();
                break;
        }
    });
}
