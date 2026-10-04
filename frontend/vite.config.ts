import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 5173,
  },
  // Relative base so the built bundle works when served from a GitHub Pages
  // project subpath (https://<user>.github.io/<repo>/) as well as from /.
  base: './',
  build: {
    outDir: 'dist',
    sourcemap: true,
  },
});