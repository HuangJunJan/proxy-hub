import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// 开发服务器固定端口 8888，把后端 API 路径代理到 7777（见 M3 prd / 项目端口约定）。
// 生产无独立前端端口：`npm run build` 产出 dist，由后端经 go:embed 单端口提供。
const backend = 'http://127.0.0.1:7777'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 8888,
    proxy: {
      '/v1': { target: backend, changeOrigin: true },
      '/admin': { target: backend, changeOrigin: true },
      '/v0': { target: backend, changeOrigin: true },
      '/healthz': { target: backend, changeOrigin: true },
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
})
