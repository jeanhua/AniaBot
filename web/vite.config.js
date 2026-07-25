import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { resolve } from 'path'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  base: './',
  build: {
    // 输出到 Go 侧 go:embed 目录，构建后提交 dist
    outDir: resolve(__dirname, '../bot/adminpanel/dist'),
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': 'http://127.0.0.1:7700',
    },
  },
})
