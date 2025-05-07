<script setup>
import { useRouter } from 'vue-router';

const router = useRouter();

function goToPhonePage() {
  router.push('/phone');
}
</script>

<template>
  <div class="home-page">
    <h1>管理电脑连接的Android设备项目</h1>
    <p>欢迎使用该项目!</p>
    <button @click="goToPhonePage">手机屏幕与文件管理</button>
  </div>
</template>

<style scoped>
.home-page {
  text-align: center;
  padding: 20px;
}
.home-page h1 {
  color: #333;
  margin-bottom: 20px;
}
.home-page p {
  color: #555;
  margin-bottom: 30px;
}
button {
  padding: 10px 20px;
  font-size: 16px;
  background-color: #5cb85c;
  color: white;
  border: none;
  border-radius: 5px;
  cursor: pointer;
  transition: background-color 0.3s;
}
button:hover {
  background-color: #4cae4c;
}
</style>