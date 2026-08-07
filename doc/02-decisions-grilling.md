# Grilling 决策全记录（50 问，逐步确认）

> 日期：2026-08-05 | 状态：Q1-Q40 已确认，Q41+ 待确认 | 方式：一次一问 + 推荐答案

## 第一域：架构与边界

### Q1 自研确认 ✅
第三方方案（Zinc 官方无 DB 同步器 / go-mysql-elasticsearch 版本不兼容 MySQL8+ES6 / Logstash 属性组装弱且无 GUI）均不满足 → **自研 Go 单二进制**。

### Q11 代码归属 ✅
**独立 git 仓库**（D:\prj\searchmiddleware 独立开发，不与车鲸鱼混仓）。

### Q12 Go 版本 ✅
`go.mod` 声明 **go 1.26**；README/CI 注明需 Go ≥ 1.26。

### Q13 端口规划 ✅
**API 与 GUI 端口分开**：API `8090`、GUI `8091`（均可配）。

### Q14 多 Zinc 实例 ✅
**v1 就支持多节点**：客户端配置 `zinc: [url1, url2]`，搜索轮询 + 节点失败自动切换（健康探测摘除）；同步 bulk 走主节点（可配）。

### Q15 配置生效方式 ✅
**热加载**：fsnotify 监听 `config/indexes/*.yaml` → 变更 → 校验 → 回灌 DB → GUI 提示"配置已变更，索引需重建"；运行参数（DSN/端口）需重启。

### Q16 日志策略 ✅
stdout 结构化 JSON（`{"ts","level","msg","index","task","dur_ms"}`）+ 文件轮转（data/logs，100MB × 10）；级别 info 默认，env 可调。

### Q17 优雅退出 ✅
捕获 SIGINT/SIGTERM → 停调度 → 等待进行中同步完成（30s 超时，超时标记 interrupted + 游标断点可续）→ 退出；退出窗口 API 返回 503。

### Q18 限流 ✅
每索引令牌桶（默认 100 QPS，`api.rate_limit` 可配）；超限 429（车鲸鱼降级 LIKE）。

### Q19 登录与鉴权 ✅（用户要求 v1 就要）
**v1 需要用户登录 + 鉴权**：API Bearer Token + GUI 账号登录（非 v1.1）。

### Q19b 账号与权限 ✅
- 用户表（元数据库 `users`）：用户名 + bcrypt；首次启动 CLI `search-service user:create admin` 引导创建
- 会话：**JWT**（`security.jwt_secret`，过期 24h 可配）；API 同 JWT 体系
- 角色两级：`admin`（全部权限）/ `viewer`（只读：查看状态/日志/对账，不能编辑配置/触发同步）
- GUI 顶部显示当前用户 + 退出登录

### Q20 版本发布 ✅
语义化版本 v0.x（配置 schema 冻结前 0.x）；git tag + `make build` 单二进制 + CHANGELOG.md（break 注明配置迁移）；独立仓库后车鲸鱼按 tag 锁定。

## 第二域：数据同步引擎

### Q2 同步模式 ✅
**批量 SQL 拉取**（非 binlog CDC）：定时增量 + 全天全量重建 + 手动触发 + **未来消息驱动增量**（用户确认："定时增量 和全天全量重建 以后可以扩展到接受消息 然后再增量"）。

### Q3/Q29 MQ 能力预留 ✅
- `POST /api/v1/notify {index, ids[]}` 幂等按 id 重建（现在实现，车鲸鱼推送后置）
- 同步内核与消息源解耦：未来 MQ 消费者只是"喂 id 给同步内核"（DocumentBuilder 复用）
- v1 不引入 MQ（YAGNI），接口与内核已就位
- 预留消费队列增量方法（与 notify 同内核）

### Q21 增量游标 ✅
存元数据库 `sync_cursors` 表（GORM）：`{index_name 主键, cursor_value 字符串, updated_at}`；成功才写回；GUI 展示游标值 + 支持手动重置。

### Q22 全量期间增量处理 ✅
索引级互斥锁（`sync_locks` 表）：增量触发时全量在跑 → 跳过本次（skipped 记录）；**全量不重置游标**（游标只前进）→ 重建窗口变更不丢。

### Q23 批量与分页 ✅
批量 500/批（`sync.batch_size` 可配）；**keyset pagination**（主键游标，非 OFFSET）：`WHERE 主键 > :last_id ORDER BY 主键 LIMIT 500`；增量同样 keyset。

### Q24 失败重试 ✅
bulk 失败重试 3 次 + 指数退避（1s/2s/4s）；失败批记录 `failed_ids`（sync_logs JSON）→ GUI **一键重试失败批**；连续 3 次失败 → ERROR + GUI 红色告警。

### Q25 软删/状态变更 ✅（用户调整）
- 状态变更：增量覆盖写（filter 排除）——无需特殊处理
- 软删清理：**每日全量重建后清理一次**（索引 id vs 库有效 id 差异删除），增量不清理——**脏数据窗口最多一天（保底可接受）**

### Q26 SQL 防护 ✅
查询超时 60s（`sync.query_timeout`）；连接池 5/数据源；**READ ONLY 事务**（快照读，线上库零锁）；慢查询日志标色（>1s 黄、>10s 红）。

### Q27 时区 ✅
统一 +08:00（DSN 显式 time_zone）；游标格式跟随库内 update_time 类型（不强制转换）；`config:check` 预检数据源时区。

### Q28 多索引并行 ✅
goroutine 池并行（默认 3 索引，`sync.max_parallel_indexes`）；同索引内互斥；忙则跳过（skipped）；GUI 显示并行状态。

### Q29 对账机制 ✅
- 一级（快速，每次增量后）：COUNT 对比（秒级）
- 二级（id 级，每日全量后）：索引全部 id（scroll）vs 库有效 id（keyset）→ 差集（缺同步/脏文档）→ **一键补同步**（重建缺的/删除多的——即 Q25 每日清理的落地）
- 结果存 `reconcile_results` 表；**每日全量兜底**（对账+清理与每日全量合并执行）

### Q30 性能监控 ✅
sync_runs 全量记录（索引/类型/触发/行数/耗时/吞吐/失败数）；索引级指标（文档数/上次同步/落后秒数/游标）；批量 P95 日志；`/api/v1/metrics` Prometheus 格式（路由预留，实现后置）。

## 第三域：索引生命周期

### Q31 命名规范 ✅
```
逻辑名: maintenance → 读别名: <env>_maintenance（GUI/API 永远用）
写索引: <env>_maintenance_write_<timestamp>（重建时新建）
环境: dev_/test_/prod_ 前缀隔离
```
GUI 展示读别名/当前写索引/历史写索引（可手动清理）。

### Q32 重建失败保障 ✅
别名原子切换（失败时 alias 不动，线上零影响）；写索引标记 failed 保留现场（GUI 重试/删除）；**校验闸门：新索引文档数/库有效行数 ≥ 0.9 才切换**。

### Q33 旧索引清理 ✅
切换成功后旧索引延迟 **24h 删除**（回退窗口，24h 内可手动回切）；最多保留 **2 个历史索引**；GUI 支持手动"立即删除/回退切换"。

### Q34 mapping 变更策略 ✅
字段/类型/分析器变更 → 检测 → 标记"索引待重建" → GUI 提示 + 一键重建（别名零停机）；**仅权重/boost 变更 → 热加载即时生效**（boost 是查询期参数）；`config:check` 输出变更检测结果。

### Q35 健康度 ✅
三色：🟢 偏差<5% 且同步成功 / 🟡 连续 1 次失败 或偏差 5-20% 或落后>1h / 🔴 连续 3 次失败 或偏差>20% 或 Zinc 不可达；动作：🟡 WARN、🔴 ERROR + `sync_alerts` 表（告警中心，webhook 预留后置）；每日对账自动刷新健康度。

## 第四域：搜索 API

### Q36 高亮 ✅
v1 支持：`highlight=1`（默认 0）；响应 `highlight: {"field": ["换<b>发动机</b>..."]}`。

### Q37/Q37b 聚合 ✅（用户要求支持）
- schema 字段 `agg: true` 标记（terms 聚合：category_ids/site_id；range 聚合：price 区间）
- 请求 `aggs` 参数：`{"categories": {"field": "category_ids", "size": 20}, "price_ranges": {"field": "price", "ranges": [[0,100],[100,300]]}}`
- 响应 `data.aggs` buckets（key/doc_count）
- GUI 搜索测试页可视化聚合；聚合字段保持不分词（keyword/numeric）

### Q38/Q38b 同义词 + 拼音 ✅（用户要求都要做）
**基于 zincsearchplusplus 实测能力定稿**：
- **拼音 = Zinc 分析器层原生支持，零开发**：env 配置 `ZINC_ANALYSIS_PINYIN_FULL=true`（全拼）+ `ZINC_ANALYSIS_PINYIN_FIRST_LETTER=true`（首字母）+ `ZINC_ANALYSIS_PINYIN_KEEP_ORIGINAL=true`（保留原词）→ "hfdj"/"shouji" 直接命中
- **同义词 = 查询期扩展**（Zinc 无 synonym filter）：同义词表（元数据库/GUI 管理）→ QueryBuilder 扩展 `bool should [原词, 同义词(boost 0.5)]`
- 分词：jieba 标准/搜索/全/NoHMM 五模式 + 词典热加载（`ZINC_ANALYSIS_USER_DICT` + ReloadJiebaBackend）
- GUI 配置页：同义词管理 Tab + 拼音开关

### Q39 搜索观测 ✅
汇总指标（QPS/Top50 关键词/耗时 P50-P99/429-500 次数，内存+定期落盘，GUI 仪表盘）；明细请求日志仅 debug 级；**不采集用户身份**（隐私）。

### Q40 热点缓存 ✅
**v1 不内置缓存**（关键词长尾命中率低；限流+降级双保险）；预留 `Cache-Control` 响应头（车鲸鱼侧/CDN 自行缓存）；文档注明走代理层缓存。

## 第五域（待确认）：车鲸鱼接入 / 部署 / 配置细节

### Q41 车鲸鱼接入（推荐，待确认）
- 仅 keyword 有值时走搜索服务；分类浏览/无关键词走数据库原逻辑（最小侵入）
- 接入点：`MaintenanceService::getPage` keyword 分支 → PHP 客户端调 `GET /api/v1/search` → `[id => score]` → 现有 DB 查询组装（展示复用）
- 降级链：不可用/超时(500ms)/429 → 日志 + 回落 LIKE
- 开关：`rescue.zinc_enabled`；新起 `shop/maintenance/search` 接口直接走搜索服务

### Q42-Q50（待确认）
部署 docker-compose 健康检查/数据卷备份/监控告警/日志采集/升级流程；环境变量优先级/密钥管理/试跑 SQL 安全/配置备份/示例索引模板。
