import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  // Собранное приложение кладётся туда, откуда его забирает go:embed,
  // чтобы получился один бинарник без внешних файлов.
  build: {
    outDir: '../internal/web/dist',
    emptyOutDir: true,
  },
  server: {
    // При разработке фронтенд ходит в локальный бэкенд.
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})
