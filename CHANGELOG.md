# Changelog

本项目遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/) 格式，版本号语义化。

## [0.1.0] - 2026-08-10

### 新增
- 骨架（Go 模块 + Vite+Vue3 GUI + embed 单二进制）
- 配置加载：app.yaml / datasources.yaml / indexes/*.yaml（热重载 + 校验 + config:check 预检）
- Zinc 客户端：多节点轮询/故障切换/ping/search/bulk/alias/index 管理
- 元数据（GORM SQLite）：users/cursors/runs/logs/schedules/reconcile/alerts/synonyms/index_configs
- JWT 鉴权（admin/viewer 角色）
- 同步引擎：文档构建器（keyset 分页/attrs 合并）、增量同步（游标/互斥/重试/失败清单）、
  全量重建（write 索引 → 90% gate → alias 原子切换 → 旧索引延迟删）、notify 按 id 重建、对账+差异修复
- 搜索 API：QueryBuilder（boost/过滤/排序/高亮/聚合透传）、限流（固定窗口 per-IP）、
  /metrics（Q39：total/耗时直方图/Top50 关键词/5xx/429）
- 定时调度：cron 全量/增量 + 启停 + next_run
- Web GUI：仪表盘/同步中心/索引编辑器/定时任务/日志/对账/同义词/告警/用户/搜索测试
- 部署：docker-compose（zinc + middleware 健康检查）、deploy.md 运维手册

### 修复
- 搜索观测指标 sm_search_total 硬编码 0（真实计数）
- searchMetrics 自死锁（writePrometheus 持锁调 topKeywords）
- alias 切换旧索引永不移除（GetAlias 按 alias 查询恒空 + 解析错误）→ 搜索命中过期数据
- 调度 getEntryID 恒 0 → next_run 永不更新
- 调度 CRUD 全链断裂（create/update/delete/toggle 不触碰 cron，删除后任务照跑）
- getExpectedCount 恒 0 → 90% 校验门禁失效（全量重建正确性校验形同虚设）

### 协作（Zinc 侧提报，15 项全部修复验证）
- BUG-002~011：mapping boost / multi_match ^ / CJK fuzziness / empty filter / SetBoost 污染 /
  numeric 聚合 key / 同义词 0 命中（BOM 根因）/ query_string·multi_match 中文 0 命中 / alias 查询 ES 语义 / ExplainStudio 401
- REQ-002/003：/api/_reload/synonym 重载 + entries 内容级热更新（裸启动静默 0）
- SUG-003/007/008/009：refresh 立即可见 / HTTP fast path / entries 覆盖语义文档化 / UI dist 过期

### 已知限制
- BUG-002 Bluge boost 为 index-time：存量文档需 re-index 才受益
- 限流按 IP 固定窗口（分布式部署需共享限流，当前单机语义）
- 待办：车鲸鱼接入（Q41，需业务确认）
