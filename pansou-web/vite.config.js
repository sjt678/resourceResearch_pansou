import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

// https://vitejs.dev/config/
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')

  // 后端地址：开发模式默认走 vite proxy，生产模式可通过 VITE_API_BASE 指定
  const apiBase = env.VITE_API_BASE || 'http://localhost:8888'

  return {
    plugins: [vue()],
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url))
      }
    },
    server: {
      port: 5173,
      host: true,
      proxy: {
        // 当 VITE_API_BASE 为空时，/api 请求走 vite 代理到后端
        '/api': {
          target: apiBase,
          changeOrigin: true
        }
      }
    },
    build: {
      outDir: 'dist',
      sourcemap: false,
      chunkSizeWarningLimit: 1500
    }
  }
})
