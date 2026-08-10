<template>
  <router-view />
</template>

<style>
/* ===== 设计系统（Soft UI Evolution）===== */
:root {
  /* 主色：teal */
  --el-color-primary: #0d9488;
  --el-color-primary-light-3: #4db6ac;
  --el-color-primary-light-5: #82cdc6;
  --el-color-primary-light-7: #b5e2de;
  --el-color-primary-light-8: #cdebe8;
  --el-color-primary-light-9: #e7f5f4;
  --el-color-primary-dark-2: #0a7670;
  /* 强调色：orange（CTA/告警高亮） */
  --app-accent: #ea580c;
  --app-accent-soft: #fef3e7;
  /* 字体 */
  --app-font: 'Fira Sans', -apple-system, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif;
  --app-font-mono: 'Fira Code', 'Consolas', 'Courier New', monospace;

  --el-border-radius-base: 8px;
  --app-sidebar-width: 220px;
  --app-page-pad: 20px;
  --app-card-gap: 16px;
  --app-transition: 200ms cubic-bezier(0.4, 0, 0.2, 1);
}

* { box-sizing: border-box; }

body {
  margin: 0;
  font-family: var(--app-font);
  background: #f5f7fa;
  -webkit-font-smoothing: antialiased;
  transition: background var(--app-transition);
}

/* ===== 页面容器与卡片 ===== */
.el-main { padding: var(--app-page-pad) !important; }

.el-card {
  border-radius: 12px;
  border: 1px solid var(--el-border-color-lighter);
  box-shadow: 0 1px 3px rgba(13, 148, 136, 0.06);
  transition: box-shadow var(--app-transition), transform var(--app-transition);
  margin-bottom: var(--app-card-gap);
}
.el-card:last-child { margin-bottom: 0; }
.el-card:hover { box-shadow: 0 6px 18px rgba(13, 148, 136, 0.1); }
.el-card__header {
  font-weight: 600;
  border-bottom: 1px solid var(--el-border-color-lighter);
  background: linear-gradient(180deg, rgba(13, 148, 136, 0.05), transparent);
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

/* ===== 表单 ===== */
.el-form-item { margin-bottom: 18px; }
.el-form-item__label { font-weight: 500; }

/* ===== 操作区 ===== */
.action-bar { display: flex; gap: 10px; margin-top: 16px; flex-wrap: wrap; }
.toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; gap: 12px; flex-wrap: wrap; }

/* ===== 分区标题 ===== */
.section-title {
  font-size: 14px;
  font-weight: 600;
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

/* ===== 代码/数值 ===== */
.mono, .el-textarea textarea, code {
  font-family: var(--app-font-mono);
}

/* ===== 按钮/标签/控件 ===== */
.el-button { border-radius: 6px; transition: all var(--app-transition); }
.el-button--primary:not(.is-plain) { box-shadow: 0 2px 6px rgba(13, 148, 136, 0.25); }
.el-button--primary:not(.is-plain):hover { box-shadow: 0 4px 12px rgba(13, 148, 136, 0.35); transform: translateY(-1px); }
.el-tag { border-radius: 6px; }
.el-input__wrapper, .el-select__wrapper { border-radius: 6px; transition: box-shadow var(--app-transition); }
.el-input__wrapper.is-focus, .el-select__wrapper.is-focused { box-shadow: 0 0 0 1px var(--el-color-primary) inset, 0 0 0 3px var(--el-color-primary-light-8); }

/* ===== 对话框 ===== */
.el-dialog { border-radius: 14px; overflow: hidden; }
.el-dialog__header { padding: 16px 20px 12px; font-weight: 600; }
.el-dialog__body { padding: 16px 20px; }
.el-dialog__footer { padding: 12px 20px 16px; border-top: 1px solid var(--el-border-color-lighter); }

/* ===== 滚动条 ===== */
::-webkit-scrollbar { width: 8px; height: 8px; }
::-webkit-scrollbar-thumb { background: #c8d0dc; border-radius: 4px; }
::-webkit-scrollbar-thumb:hover { background: #aab4c4; }
::-webkit-scrollbar-track { background: transparent; }

/* ===== 空状态 ===== */
.el-empty { padding: 40px 0; }

/* ===== 暗色模式（Soft UI Dark）===== */
html.dark {
  --el-bg-color: #171a1f;
  --el-bg-color-page: #101318;
  --el-bg-color-overlay: #1d2128;
  --el-text-color-primary: #e6e9ee;
  --el-text-color-regular: #c3c9d2;
  --el-text-color-secondary: #8b93a0;
  --el-border-color: #2a2f38;
  --el-border-color-light: #242932;
  --el-border-color-lighter: #1f242c;
  --el-fill-color: #22262e;
  --el-fill-color-light: #1e2229;
  --el-fill-color-lighter: #1a1e24;
  --app-accent-soft: #2a1f14;
}
html.dark body { background: var(--el-bg-color-page); }
html.dark .el-main { background: var(--el-bg-color-page); }
html.dark .el-card { background: var(--el-bg-color); box-shadow: 0 1px 3px rgba(0, 0, 0, 0.5); }
html.dark .el-card:hover { box-shadow: 0 6px 18px rgba(0, 0, 0, 0.6); }
html.dark .el-card__header { border-bottom-color: var(--el-border-color-lighter); }
html.dark .el-table th.el-table__cell { background: #20242b !important; }
html.dark .el-table .el-table__row:hover > td { background: #1d242b !important; }
html.dark ::-webkit-scrollbar-thumb { background: #3a3f48; }
html.dark .el-dialog { background: var(--el-bg-color-overlay); }
</style>
