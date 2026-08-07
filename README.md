# Search Middleware（搜索中间件）

MySQL ↔ ZincSearch 双向索引同步中间件：SQL 配置驱动文档组装、零停机全量重建、增量游标同步、搜索 API、JWT 鉴权、定时调度。Go 单二进制。

## 快速开始

```bash
# 0. 一键构建（前端 + Go embed + 单二进制）
powershell -ExecutionPolicy Bypass -File build.ps1

# 1. 配置（config/app.yaml 的 zinc 账号、datasources.yaml 的 MySQL DSN）
# 2. 创建管理员
./searchmiddleware.exe user:create admin admin123 admin

# 3. 启动（API :8090 + GUI :8091）
./searchmiddleware.exe -config config/app.yaml

# 4. 打开 Web GUI（浏览器访问，/api 自动代理到 8090）
http://localhost:8091

# 5. 登录拿 token（供 API 调用）
curl -X POST http://localhost:8090/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}'

# 6. 搜索（index=逻辑索引名）
curl 'http://localhost:8090/api/v1/search?index=maintenance&keyword=发动机&limit=10' \
  -H "Authorization: Bearer $TOKEN"
```

## Web GUI（Vue3 + Element Plus，embed 单二进制）

| 页面 | 功能 |
|------|------|
| 仪表盘 | 索引卡片/健康度/运行状态 |
| 同步中心 | 手动全量/增量/指定ID、对账 COUNT/ID 级触发 |
| 定时任务 | 新建/删除 cron 任务 |
| 日志中心 | 按索引/级别筛选 |
| 对账中心 | 差异清单 + 一键补同步 |
| 索引配置 | YAML 编辑器（新建/保存/删除，保存后提示重建） |
| 同义词 | 增删（查询期即时生效） |
| 告警中心 | 告警列表 |
| 用户管理 | admin 新建用户 |

- 前端源码 `web-ui/`（Vite + Vue3 + Element Plus + Pinia + vue-router hash 路由）
- 构建产物 embed 进 `internal/web/dist`（`//go:embed`），单二进制自包含
- GUI 端口 `/api/*` 自动反向代理到 API 端口（Q13 端口分离 + 代理桥接）
- 开发模式：`cd web-ui && npm run dev`（vite proxy 到 8090）

## 目录结构

```
cmd/server/            # 入口：API + 调度 + 优雅退出 + user:create
internal/
├── config/            # app.yaml + datasources.yaml + indexes/*.yaml（SHA256 版本）
├── metadata/          # GORM 元数据：users/cursors/runs/logs/schedules/alerts/synonyms
├── zinc/              # 多节点客户端：轮询/故障切换/search/bulk/alias 原子切换
├── indexer/           # SQL→文档构建器：base SQL + attrs SQL 合并、keyset 分页、array 转换
├── sync/              # 同步引擎：全量（90% 闸门）/增量（游标）/by_ids/失败重试
├── lifecycle/         # 索引生命周期：write 索引/别名原子切换/旧索引延迟清理
├── query/             # 查询编排：权重 boost/同义词扩展/过滤/分页/排序/高亮/聚合
├── api/               # HTTP API：/search /health /notify /api/v1/*（JWT + 角色）
├── scheduler/         # cron 调度：默认增量 5min + 每日全量 02:00
└── auth/              # JWT 签发/校验、bcrypt、admin/viewer 角色
config/
├── app.yaml           # 端口/密钥/zinc 集群/同步参数
├── datasources.yaml   # 多 MySQL 数据源注册
└── indexes/*.yaml     # 索引配置（唯一真相）：maintenance.yaml 为示例
```

## 核心机制

| 机制 | 设计 |
|------|------|
| 零停机重建 | 新建 `<env>_<idx>_write_<ts>` → 全量 → 90% 校验 → AliasSwap 原子切换 → 旧索引 24h 后清理 |
| 增量同步 | `update_time > :cursor` + keyset 分页，游标存 `sync_cursors`，乐观写入（先 Zinc 后游标） |
| 多数据源 | 每索引独立引用 `datasources.yaml` 的数据源（不支持单索引聚合多库，v1 范围） |
| 多 Zinc 节点 | 轮询 + 实际探测故障切换，bulk/search 自动路由 |
| 配置真相 | 文件唯一真相，内容 SHA256 版本标识 |
| 鉴权 | JWT Bearer，admin（全权限）/viewer（只读），登录 24h 过期 |

## 索引配置（config/indexes/*.yaml）

```yaml
source:
  datasource: main            # 引用 datasources.yaml
  sql_query: "SELECT ... FROM shop_maintenance WHERE delete_time = 0"
  sql_joined_field:           # 属性 SQL（GROUP_CONCAT 合并）
    category_names: "SELECT ..."
  incremental_field: update_time

index:
  name: maintenance
  analyzer: jieba_std
  search_analyzer: jieba_search
  boost: { maintenance_name: 5.0, sub_title: 3.0 }
  fields:
    maintenance_name: { type: text, searchable: true }
    category_ids: { type: keyword, element_type: long, filter: true, agg: true }
    price: { type: float, sortable: true, agg: true }
    update_time: { type: date, format: unix_timestamp, sortable: true }
```

字段类型映射 Zinc：`text`（分词搜索/高亮）、`keyword`（精确过滤/排序/聚合）、`numeric`/`float`、`date`、`element_type`（数组元素类型，如 `category_ids`）。

## API 摘要

| 端点 | 说明 |
|------|------|
| `GET /health` | 健康检查（含 Zinc 连通性） |
| `POST /api/v1/auth/login` | 登录签发 JWT |
| `GET /api/v1/search` | 搜索（keyword/filter/sort/highlight/aggs/site_id） |
| `POST /api/v1/notify` | `{index, ids[]}` 幂等按 id 重建（MQ 预留） |
| `POST /api/v1/indexes/{name}/sync` | 手动触发 full/incremental/by_ids |
| `POST /api/v1/indexes/{name}/reconcile?type=count\|id` | 对账（count 秒级 / id 级差集） |
| `GET /api/v1/indexes/{name}/reconcile` | 对账结果历史 |
| `POST /api/v1/indexes/{name}/reconcile/{id}/fix` | 一键补同步（重建缺的 + 删脏的） |
| `GET /api/v1/metrics` | Prometheus 文本格式指标 |
| `GET /api/v1/runs` `GET /api/v1/logs` | 同步历史/日志 |
| `/api/v1/schedules` `/api/v1/synonyms` `/api/v1/users` `/api/v1/alerts` | 管理接口（admin） |

错误码：`0` 成功 / `40401` 索引不存在 / `40001` 参数非法 / `42901` 限流 / `50001` Zinc 不可用 / `40101` 未认证 / `40301` 权限不足。

## 运维命令

```bash
# 创建用户（首次启动引导）
./searchmiddleware.exe user:create <username> <password> [admin|viewer]

# 配置预检：全量校验 + 数据源连通 + 索引版本(SHA256) + Zinc 连通
./searchmiddleware.exe config:check
```

## 测试

```bash
go test ./...   # 配置解析/索引器组装/JWT/元数据 CRUD/Zinc mock（含故障转移）
```

## 需求文档

见 `doc/`：需求总览、Grilling 决策记录（Q1-Q64 已确认）、架构、索引 schema、API 契约、GUI 设计、Zinc 能力调研、路线图。
