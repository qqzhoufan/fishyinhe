<script setup>
// 这里可以保留或移除之前测试 API 的代码，根据你的需要
// 如果要保留，可以将其逻辑移到 HomePage.vue 或其他特定页面
</script>

<template>
  <div id="app-layout">
    <header class="app-header">
      <nav>
        <router-link to="/">主页</router-link> |
        <router-link to="/phone">手机功能</router-link>
      </nav>
    </header>
    <main class="app-content">
      <router-view /> </main>
    <footer class="app-footer">
      <p>&copy; {{ new Date().getFullYear() }} 我的项目</p>
    </footer>
  </div>
</template>

<style scoped>
#app-layout {
  display: flex;
  flex-direction: column;
  min-height: 100vh;
  font-family: Avenir, Helvetica, Arial, sans-serif;
}

.app-header {
  background-color: #333;
  padding: 15px 30px;
  text-align: center;
}

.app-header nav a {
  font-weight: bold;
  color: #ddd;
  margin: 0 15px;
  text-decoration: none;
  font-size: 1.1em;
}

.app-header nav a.router-link-exact-active {
  color: #42b983; /* Vue Green for active link */
}

.app-content {
  flex-grow: 1;
  padding: 20px;
  /* background-color: #f4f4f4; */
}

.app-footer {
  text-align: center;
  padding: 15px;
  background-color: #333;
  color: #ccc;
  font-size: 0.9em;
}
</style>