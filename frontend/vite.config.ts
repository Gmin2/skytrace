import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
  ],
  server: {
    proxy: {
      '/api': 'http://167.71.231.68:8080',
      '/ws': {
        target: 'ws://167.71.231.68:8080',
        ws: true,
      },
    },
  },
})
