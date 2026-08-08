<template>
  <router-view />
</template>

<style>
body {
  margin: 0;
  font-family: -apple-system, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif;
}

html.dark body {
  background: #141414;
}

/* 暗色下页面主区与卡片统一底色 */
html.dark .el-main {
  background: #141414;
}
html.dark .el-card {
  background: #1d1e1f;
}
html.dark .el-card__header {
  border-bottom-color: #2c2c2d;
}
</style>
