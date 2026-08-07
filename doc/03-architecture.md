# 架构设计

> 日期：2026-08-05 | 项目：D:\prj\searchmiddleware

## 总体架构

```
车鲸鱼业务（PHP，改造调 /search）           Search Middleware（Go 单二进制）
┌─────────────────────┐   HTTP /search    ┌──────────────────────────────────┐
│ 维修项目搜索接口        │ ────────────────▶ │  HTTP API (:8090)               │
│（keyword 走搜索服务，   │ ◀──────────────── │   /api/v1/search  /health      │
│ 失败降级 LIKE）        │   JSON            │   /api/v1/notify  /api/v1/*    │
└─────────────────────┘                   └──────────────┬───────────────────┘
                                                         │
                                        ┌────────────────┼──────────────────┐
                                        │                │                  │
                                   ┌────▼────┐    ┌──────▼──────┐   ┌──────▼──────┐
                                   │ Web GUI │    │ 同步引擎     │   │ 定时调度     │
                                   │ (:8091) │    │ 全量/增量/   │   │ cron 全量/   │
                                   │ Vue3+EP │    │ 消息驱动预留  │   │ 增量周期     │
                                   └─────────┘    └──────┬──────┘   └─────────────┘
                                                         │ SQL 配置驱动（YAML）
                                        ┌────────────────▼────────────────┐
                                        │ ZincSearch（zincsearchplusplus） │
                                        │ docker-compose，多实例可配        │
                                        └────────────────┬────────────────┘
                                                         │
                                        ┌────────────────▼────────────────┐
                                        │ MySQL（多数据源 datasources）     │
                                        └─────────────────────────────────┘
```

## 模块划分（internal/）

```
cmd/server/main.go          # 入口：API + GUI + 调度 + 优雅退出
internal/
├── config/                 # 配置加载（app.yaml 多环境 + indexes/*.yaml + datasources.yaml）
├── metadata/               # GORM 元数据模型：users/cursors/runs/logs/schedules/reconcile/alerts/synonyms/index_configs
├── zinc/                   # Zinc 客户端：多节点轮询/故障切换/search/bulk/alias/index 管理
├── indexer/                # SQL→文档 构建器（base SQL + attrs SQL 合并，keyset 分页）
├── sync/                   # 同步引擎：全量（别名零停机）/增量（游标）/notify/重试/对账清理
├── lifecycle/              # 索引生命周期：创建 write 索引/alias 原子切换/旧索引清理/90% 闸门
├── query/                  # 查询编排：权重 boost/同义词扩展/过滤/分页/排序/高亮/聚合
├── api/                    # HTTP API：/search /health /notify /api/v1/*（JWT 鉴权 + 限流）
├── scheduler/              # cron 调度：全量/增量周期、索引并行池、互斥
├── web/                    # GUI 静态托管（embed Vue 构建产物）
└── auth/                   # JWT 签发/校验、bcrypt、admin/viewer 角色
```

## 关键机制

| 机制 | 设计 |
|------|------|
| 零停机重建 | 写索引 `_write_<ts>` → 全量 → 90% 校验 → alias 原子切换 → 旧索引延迟 24h 删 |
| 增量 | update_time 游标（sync_cursors）+ keyset 分页；全量不重置游标 |
| 互斥 | sync_locks 表（同索引全量/增量互斥，忙则跳过） |
| 降级 | 车鲸鱼侧：搜索服务不可用/超时/429 → LIKE（PHP 侧） |
| 多环境 | SEARCH_ENV（dev/test/prod）+ 索引前缀 + 独立配置 |
| 多数据源 | datasources.yaml 注册多个 MySQL，索引定义 `datasource` 引用 |
| 多 Zinc | zinc 节点列表轮询 + 健康探测摘除 |
| 配置真相 | 文件唯一真相：GUI 编辑 → DB（展示）→ 原子写文件；启动/变更回灌 DB；冲突以文件为准 |
| 安全 | JWT 登录（admin/viewer）、API Bearer、SELECT 白名单试跑、内网部署 + API_TOKEN 可选 |

## 部署（docker-compose）

```yaml
services:
  zinc:            # zincsearchplusplus（jieba/拼音/词典 env 配置）
  searchmiddleware: # 单二进制（API+GUI+调度），挂载 config/ data/ 卷
```

## 数据流示例（maintenance 索引）

```
定时增量(5min) → 读游标 → base SQL(update_time > cursor) + attrs SQL(分类名/SKU名)
  → 文档组装 → bulk 到 <env>_maintenance（读别名）→ 更新游标 → COUNT 对账
每日全量(02:00) → 新建 <env>_maintenance_write_<ts> → 全量拉取+组装 → bulk
  → 90% 校验 → alias 切换 → 旧索引延迟删 → id 级对账 + 软删清理
消息驱动(未来) → POST /notify {index, ids} → 单文档重建（同 DocumentBuilder）
```
