import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5680, // 在这里设置你想要的端口号
    // open: true, // 可选：如果想让 Vite 自动在浏览器中打开应用
  }
})