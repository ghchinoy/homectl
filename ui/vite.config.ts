import { defineConfig } from 'vite';
import path from 'node:path';

export default defineConfig({
  resolve: {
    dedupe: ['lit', '@lit/reactive-element', 'lit-html', 'lit-element'],
  },
  server: {
    fs: {
      allow: [
        path.resolve(__dirname, '.'),
        path.resolve(__dirname, '../../lustrous-lit'),
      ],
    },
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
});
