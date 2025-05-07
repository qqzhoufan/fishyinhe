// D:\fishyinhe\frontend\src\router\index.js
import { createRouter, createWebHistory } from 'vue-router';
import HomePage from '../views/HomePage.vue'; // 我们稍后会创建这个文件
import PhonePage from '../views/PhonePage.vue'; // 我们稍后会创建这个文件

const routes = [
    {
        path: '/',
        name: 'Home',
        component: HomePage,
    },
    {
        path: '/phone',
        name: 'Phone',
        component: PhonePage,
        // 示例：props: true, // 如果需要通过路由传递 props
    },
    // 未来可以添加更多路由
    // {
    //   path: '/about',
    //   name: 'About',
    //   component: () => import('../views/AboutPage.vue') // 懒加载示例
    // }
];

const router = createRouter({
    history: createWebHistory(import.meta.env.BASE_URL), // 使用 HTML5 History 模式
    routes,
});

export default router;