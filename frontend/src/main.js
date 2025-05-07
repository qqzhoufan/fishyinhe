// D:\fishyinhe\frontend\src\main.js
import {createApp} from 'vue'
import './style.css' // Vite 项目通常会有一个全局样式文件
import App from './App.vue'
import router from './router' // 导入路由配置

const app = createApp(App)

app.use(router) // 使用路由插件

app.mount('#app')