import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  // The built app lands where go:embed picks it up, so that a single binary
  // comes out with no external files.
  build: {
    outDir: '../internal/web/dist',
    emptyOutDir: true,
  },
  server: {
    // During development the frontend talks to the local backend.
    proxy: {
      '/api': 'http://localhost:58080',
    },
  },
})
