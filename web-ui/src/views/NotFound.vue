<template>
  <div class="nf-wrap">
    <div class="nf-code mono">404</div>
    <div class="nf-title">页面不存在</div>
    <div class="nf-desc">你访问的页面已被移动或不存在</div>
    <el-button type="primary" @click="$router.push('/')">返回仪表盘</el-button>
  </div>
</template>

<style scoped>
.nf-wrap {
  height: 100vh;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  background: var(--el-bg-color-page);
}
.nf-code {
  font-size: 72px;
  font-weight: 700;
  color: var(--el-color-primary);
  opacity: 0.8;
  text-shadow: 0 0 24px rgba(13, 148, 136, 0.25);
}
.nf-title { font-size: 20px; font-weight: 600; color: var(--el-text-color-primary); }
.nf-desc { font-size: 14px; color: var(--el-text-color-secondary); margin-bottom: 12px; }
html.dark .nf-code { text-shadow: 0 0 24px rgba(34, 211, 238, 0.35); }
</style>
