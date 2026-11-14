import { defineConfig } from 'vite';

export default defineConfig({
  base: './',
  esbuild: { jsx: 'automatic' },
  build: {
    assetsDir: '_vite',
    cssCodeSplit: false,
    sourcemap: false,
  },
});
