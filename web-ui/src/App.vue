<template>
  <router-view />
</template>

<style>
:root {
  --el-color-primary: #3b6cf6;
  --el-color-primary-light-3: #6b8ef8;
  --el-color-primary-light-5: #93b0fa;
  --el-color-primary-light-7: #bcd1fc;
  --el-color-primary-light-8: #d3e0fd;
  --el-color-primary-light-9: #e9effe;
  --el-color-primary-dark-2: #2f56c4;
  --el-border-radius-base: 8px;
  --app-sidebar-width: 220px;
  --app-page-pad: 20px;
  --app-card-gap: 16px;
}

* { box-sizing: border-box; }

body {
  margin: 0;
  font-family: -apple-system, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif;
  background: #f5f7fa;
  -webkit-font-smoothing: antialiased;
}

/* ===== 页面容器与卡片 ===== */
.el-main { padding: var(--app-page-pad) !important; }

.el-card {
  border-radius: 12px;
  border: 1px solid var(--el-border-color-lighter);
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.04);
  transition: box-shadow 0.2s ease;
  margin-bottom: var(--app-card-gap);
}
.el-card:last-child { margin-bottom: 0; }
.el-card:hover { box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08); }
.el-card__header {
  font-weight: 600;
  border-bottom: 1px solid var(--el-border-color-lighter);
  background: linear-gradient(180deg, rgba(59, 108, 246, 0.04), transparent);
  padding: 14px 20px;
}
.el-card__body { padding: 20px; }

/* ===== 表格 ===== */
.el-table { border-radius: 8px; overflow: hidden; }
.el-table th.el-table__cell {
  background: #f7f9fc !important;
  font-weight: 600;
  color: var(--el-text-color-primary);
}
.el-table .el-table__row:hover > td { background: var(--el-color-primary-light-9) !important; }

/* ===== 表单：间距与对齐 ===== */
.el-form-item { margin-bottom: 18px; }
.el-form-item__label { font-weight: 500; }

/* ===== 按钮区 ===== */
.action-bar {
  display: flex;
  gap: 10px;
  margin-top: 16px;
  flex-wrap: wrap;
}
.toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  gap: 12px;
  flex-wrap: wrap;
}

/* ===== 分区标题 ===== */
.section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--el-text-color-primary);
  margin: 8px 0 12px;
  display: flex;
  align-items: center;
  gap: 8px;
}
.section-title::before {
  content: '';
  width: 3px;
  height: 14px;
  border-radius: 2px;
  background: var(--el-color-primary);
}

/* ===== 滚动条 ===== */
::-webkit-scrollbar { width: 8px; height: 8px; }
::-webkit-scrollbar-thumb { background: #c8d0dc; border-radius: 4px; }
::-webkit-scrollbar-thumb:hover { background: #aab4c4; }
::-webkit-scrollbar-track { background: transparent; }

/* ===== 按钮与标签 ===== */
.el-button { border-radius: 6px; }
.el-button--primary:not(.is-plain) { box-shadow: 0 2px 6px rgba(59, 108, 246, 0.25); }
.el-tag { border-radius: 6px; }

/* ===== 对话框 ===== */
.el-dialog { border-radius: 14px; overflow: hidden; }
.el-dialog__header { padding: 16px 20px 12px; font-weight: 600; }
.el-dialog__body { padding: 16px 20px; }
.el-dialog__footer { padding: 12px 20px 16px; border-top: 1px solid var(--el-border-color-lighter); }

/* ===== 输入控件 ===== */
.el-input__wrapper, .el-select__wrapper { border-radius: 6px; }

/* ===== 空状态 ===== */
.el-empty { padding: 40px 0; }

/* ===== 暗色模式 ===== */
html.dark body { background: #141414; }
html.dark .el-main { background: #141414; }
html.dark .el-card { background: #1d1e1f; box-shadow: 0 1px 3px rgba(0,0,0,0.5); }
html.dark .el-card__header { border-bottom-color: #2c2c2d; }
html.dark .el-table th.el-table__cell { background: #232425 !important; }
html.dark .el-table .el-table__row:hover > td { background: #26272a !important; }
html.dark ::-webkit-scrollbar-thumb { background: #3a3b3c; }
</style>
