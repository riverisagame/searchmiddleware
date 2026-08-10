<template>
  <router-view />
</template>

<style>
/* ===== 设计系统：Cyber Tech（暗色科技感 + 亮色可用）===== */
:root {
  --el-color-primary: #0d9488;
  --el-color-primary-light-3: #4db6ac;
  --el-color-primary-light-5: #82cdc6;
  --el-color-primary-light-7: #b5e2de;
  --el-color-primary-light-8: #cdebe8;
  --el-color-primary-light-9: #e7f5f4;
  --el-color-primary-dark-2: #0a7670;
  --app-accent: #ea580c;
  --app-accent-soft: #fef3e7;
  --app-font: -apple-system, 'Segoe UI', 'PingFang SC', 'Microsoft YaHei', sans-serif;
  --app-font-mono: 'Fira Code', 'Share Tech Mono', 'Consolas', 'Courier New', monospace;

  --el-border-radius-base: 8px;
  --app-sidebar-width: 230px;
  --app-page-pad: 24px;
  --app-card-gap: 20px;
  --app-transition: 200ms cubic-bezier(0.4, 0, 0.2, 1);
  --app-table-row-h: 48px;
}

* { box-sizing: border-box; }

body {
  margin: 0;
  font-family: var(--app-font);
  background: #f5f7fa;
  -webkit-font-smoothing: antialiased;
  transition: background var(--app-transition);
}

/* ===== 页面与卡片 ===== */
.el-main { padding: var(--app-page-pad) !important; }

.el-card {
  border-radius: 14px;
  border: 1px solid var(--el-border-color-lighter);
  box-shadow: 0 1px 3px rgba(13, 148, 136, 0.06);
  transition: box-shadow var(--app-transition), transform var(--app-transition);
  margin-bottom: var(--app-card-gap);
}
.el-card:last-child { margin-bottom: 0; }
.el-card:hover { box-shadow: 0 6px 18px rgba(13, 148, 136, 0.1); }
.el-card__header {
  font-weight: 600;
  font-size: 15px;
  border-bottom: 1px solid var(--el-border-color-lighter);
  background: linear-gradient(180deg, rgba(13, 148, 136, 0.05), transparent);
  padding: 16px 22px;
}
.el-card__body { padding: 22px; }

/* ===== 表格：大行高 + 可读性 ===== */
.el-table { border-radius: 10px; overflow: hidden; font-size: 13.5px; }
.el-table th.el-table__cell {
  background: #f7f9fc !important;
  font-weight: 600;
  padding: 12px 0;
}
.el-table td.el-table__cell { padding: 12px 0; height: var(--app-table-row-h); }
.el-table .el-table__row:hover > td { background: var(--el-color-primary-light-9) !important; }
.el-table--striped .el-table__body tr.el-table__row--striped td.el-table__cell { background: rgba(13, 148, 136, 0.03); }

/* ===== 交互反馈 ===== */
.el-button, .el-menu-item, .el-checkbox, .el-radio, .el-select, .el-input, .el-tag, .el-table__row, .el-dropdown {
  cursor: pointer;
}
.el-button:active { transform: scale(0.97); }

/* ===== 状态标签 ===== */
.el-tag { font-weight: 500; letter-spacing: 0.2px; }
html.dark .el-tag--success { background: rgba(34, 197, 94, 0.15); color: #4ade80; border-color: rgba(34, 197, 94, 0.3); }
html.dark .el-tag--danger { background: rgba(239, 68, 68, 0.15); color: #f87171; border-color: rgba(239, 68, 68, 0.3); }
html.dark .el-tag--warning { background: rgba(245, 158, 11, 0.15); color: #fbbf24; border-color: rgba(245, 158, 11, 0.3); }
html.dark .el-tag--info { background: rgba(148, 163, 184, 0.12); color: #94a3b8; border-color: rgba(148, 163, 184, 0.25); }

/* ===== 数值列等宽 ===== */
.mono-num { font-family: var(--app-font-mono); font-variant-numeric: tabular-nums; }

/* ===== 数字滚动/发光（暗色科技感）===== */
html.dark .stat-value { text-shadow: 0 0 12px rgba(34, 211, 238, 0.25); }

/* ===== 表单：更大控件与间距 ===== */
.el-form-item { margin-bottom: 20px; }
.el-form-item__label { font-weight: 500; font-size: 14px; }
.el-input__wrapper, .el-select__wrapper { border-radius: 8px; min-height: 36px; }
.el-textarea__inner { font-size: 13.5px; }

/* ===== 按钮：更大 ===== */
.el-button { border-radius: 8px; min-height: 32px; padding: 8px 16px; transition: all var(--app-transition); }
.el-button--primary:not(.is-plain) { box-shadow: 0 2px 6px rgba(13, 148, 136, 0.25); }
.el-button--primary:not(.is-plain):hover { box-shadow: 0 4px 12px rgba(13, 148, 136, 0.35); transform: translateY(-1px); }
.el-button + .el-button { margin-left: 10px; }
.el-tag { border-radius: 6px; padding: 0 10px; height: 26px; line-height: 24px; }

/* ===== 操作区 ===== */
.action-bar { display: flex; gap: 12px; margin-top: 18px; flex-wrap: wrap; }
.toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 18px; gap: 12px; flex-wrap: wrap; }

/* ===== 分区标题 ===== */
.section-title {
  font-size: 15px;
  font-weight: 600;
  margin: 10px 0 14px;
  display: flex;
  align-items: center;
  gap: 8px;
}
.section-title::before {
  content: '';
  width: 3px;
  height: 16px;
  border-radius: 2px;
  background: var(--el-color-primary);
}

/* ===== 代码/数值 ===== */
.mono, .el-textarea textarea, code {
  font-family: var(--app-font-mono);
}

/* ===== 对话框 ===== */
.el-dialog { border-radius: 16px; overflow: hidden; }
.el-dialog__header { padding: 18px 22px 14px; font-weight: 600; font-size: 15px; }
.el-dialog__body { padding: 18px 22px; }
.el-dialog__footer { padding: 14px 22px 18px; border-top: 1px solid var(--el-border-color-lighter); }

/* ===== 滚动条 ===== */
::-webkit-scrollbar { width: 10px; height: 10px; }
::-webkit-scrollbar-thumb { background: #c8d0dc; border-radius: 5px; }
::-webkit-scrollbar-thumb:hover { background: #aab4c4; }

/* ===== 空状态 ===== */
.el-empty { padding: 48px 0; }

/* ===== 暗色模式：Cyber Tech ===== */
html.dark {
  --el-color-primary: #22d3ee;
  --el-color-primary-light-3: #4fe0f3;
  --el-color-primary-light-5: #7ceaf7;
  --el-color-primary-light-7: #b0f4fb;
  --el-color-primary-light-8: #cdf8fd;
  --el-color-primary-light-9: #e7fcfe;
  --el-color-primary-dark-2: #0ea5be;
  --app-accent: #22c55e;
  --app-accent-soft: #14241a;
  --app-neon: #22d3ee;
  --el-bg-color: #161d2e;
  --el-bg-color-page: #0f172a;
  --el-bg-color-overlay: #1c2438;
  --el-text-color-primary: #e7ecf5;
  --el-text-color-regular: #c2cad9;
  --el-text-color-secondary: #8494ab;
  --el-border-color: #2c3852;
  --el-border-color-light: #273349;
  --el-border-color-lighter: #212c41;
  --el-fill-color: #1e2940;
  --el-fill-color-light: #1a2438;
  --el-fill-color-lighter: #171f31;
}
html.dark body { background: var(--el-bg-color-page); }
html.dark .el-main { background: var(--el-bg-color-page); }
html.dark .el-card {
  background: linear-gradient(180deg, rgba(34, 211, 238, 0.03), transparent 60%), var(--el-bg-color);
  border-color: var(--el-border-color-lighter);
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.5);
}
html.dark .el-card:hover { box-shadow: 0 6px 20px rgba(0, 0, 0, 0.65), 0 0 0 1px rgba(34, 211, 238, 0.15); }
html.dark .el-card__header {
  border-bottom-color: var(--el-border-color-lighter);
  background: linear-gradient(180deg, rgba(34, 211, 238, 0.06), transparent);
}
html.dark .el-table th.el-table__cell { background: #1b2437 !important; }
html.dark .el-table .el-table__row:hover > td { background: #182136 !important; }
html.dark .el-table { --el-table-border-color: var(--el-border-color-lighter); --el-table-header-bg-color: transparent; }
html.dark ::-webkit-scrollbar-thumb { background: #33405c; }
html.dark .el-dialog { background: var(--el-bg-color-overlay); }
html.dark .el-button--primary:not(.is-plain) {
  background: linear-gradient(90deg, #0ea5be, #22d3ee);
  border-color: transparent;
  box-shadow: 0 2px 10px rgba(34, 211, 238, 0.25);
}
html.dark .el-button--primary:not(.is-plain):hover { box-shadow: 0 4px 16px rgba(34, 211, 238, 0.4); }
html.dark .el-input__wrapper.is-focus, html.dark .el-select__wrapper.is-focused {
  box-shadow: 0 0 0 1px var(--el-color-primary) inset, 0 0 0 3px rgba(34, 211, 238, 0.15);
}
</style>
