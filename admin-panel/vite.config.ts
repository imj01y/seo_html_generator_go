import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import { ElementPlusResolver } from 'unplugin-vue-components/resolvers'

export default defineConfig({
  base: '/',
  plugins: [
    vue(),
    AutoImport({
      imports: ['vue', 'vue-router', 'pinia'],
      resolvers: [ElementPlusResolver()],
      dts: 'src/auto-imports.d.ts',
    }),
    Components({
      resolvers: [ElementPlusResolver()],
      dts: 'src/components.d.ts',
    }),
  ],
  resolve: {
    alias: {
      '@': resolve(__dirname, 'src'),
    },
  },
  server: {
    port: 3000,
    proxy: {
      // WebSocket 代理需要单独配置
      '/api/crawl/test-spider-code-ws': {
        target: 'http://localhost:8000',
        changeOrigin: true,
        ws: true,
      },
      // 数据源执行日志 WebSocket
      '/api/crawl/sources': {
        target: 'http://localhost:8000',
        changeOrigin: true,
        ws: true,
      },
      // 爬虫项目执行日志 WebSocket
      '/api/spider-projects': {
        target: 'http://localhost:8000',
        changeOrigin: true,
        ws: true,
      },
      // 系统日志 WebSocket
      '/api/logs/ws': {
        target: 'http://localhost:8000',
        changeOrigin: true,
        ws: true,
      },
      '/api': {
        target: 'http://localhost:8000',
        changeOrigin: true,
      },
    },
  },
  optimizeDeps: {
    include: ['monaco-editor'],
  },
})
