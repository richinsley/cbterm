import { defineConfig } from 'vite';

export default defineConfig({
    build: {
        outDir: 'dist',
        assetsDir: 'assets',
        // Inline everything for single-file output
        assetsInlineLimit: 100000,
        rollupOptions: {
            output: {
                // Single JS file
                manualChunks: undefined,
            },
        },
    },
    server: {
        port: 3000,
        proxy: {
            '/ws': {
                target: 'ws://localhost:8080',
                ws: true,
            },
        },
    },
});
