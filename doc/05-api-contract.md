# API 契约

> 日期：2026-08-05 | 端口：API 8090 / GUI 8091（可配）| 鉴权：JWT Bearer（v1 必需）

## 搜索接口

### `GET /api/v1/search`

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `index` | string | ✅ | 逻辑索引名（自动加环境前缀） |
| `keyword` | string | ⚠️ | 与 filter 二选一，均无 = match_all |
| `site_id` | int | ❌ | 站点隔离（terms filter [0, site_id]） |
| `filter` | json | ❌ | `{"status":1,"category_ids":[238],"price":{"gte":10,"lte":100}}` |
| `page` | int | ❌ | 默认 1 |
| `limit` | int | ❌ | 默认 10，最大 100 |
| `sort` | string | ❌ | `score`（默认）/ `sort:desc` / `price:asc` |
| `highlight` | int | ❌ | 1 开启（字段级片段） |
| `aggs` | json | ❌ | `{"categories":{"field":"category_ids","size":20},"price_ranges":{"field":"price","ranges":[[0,100],[100,300]]}}` |

### 响应

```json
{
  "code": 0,
  "msg": "ok",
  "data": {
    "total": 28,
    "page": 1,
    "limit": 10,
    "items": [
      {
        "id": 11073,
        "score": 8.42,
        "fields": { "maintenance_id": 11073, "maintenance_name": "换发动机后胶垫" },
        "highlight": { "maintenance_name": ["换<b>发动机</b>后胶垫"] }
      }
    ],
    "aggs": {
      "categories": { "buckets": [ { "key": 238, "doc_count": 12 } ] },
      "price_ranges": { "buckets": [ { "key": "0-100", "doc_count": 5 } ] }
    }
  }
}
```

**约定**：`items[].fields` 返回文档全部字段；车鲸鱼拿 `id` 列表回库查详情（展示走 DB，Zinc 只管候选集与排序）。

## 错误码

| code | 含义 | HTTP |
|------|------|------|
| 0 | 成功 | 200 |
| 40401 | 索引不存在/未定义 | 404 |
| 40001 | 参数非法（filter 结构/limit 超界） | 400 |
| 42901 | 限流（超 QPS） | 429 |
| 50001 | Zinc 不可用/查询异常 | 503 |
| 40101 | 未认证/Token 失效 | 401 |
| 40301 | 权限不足（viewer 操作管理接口） | 403 |

**降级契约**：车鲸鱼侧收到 503/429/超时（500ms）→ 记录降级日志 → 回落 LIKE。

## 其他端点

| 端点 | 方法 | 说明 |
|------|------|------|
| `/health` | GET | 健康检查（含 Zinc 连通性） |
| `/api/v1/notify` | POST | `{index, ids[]}` 幂等按 id 重建（MQ 预留入口） |
| `/api/v1/indexes` | GET/POST/PUT/DELETE | 索引定义管理（GUI 配置编辑器后端） |
| `/api/v1/indexes/{name}/sync` | POST | 手动触发全量/增量/指定 id |
| `/api/v1/indexes/{name}/reconcile` | POST/GET | 对账触发与结果 |
| `/api/v1/runs` / `/api/v1/logs` | GET | 任务历史/日志中心 |
| `/api/v1/schedules` | GET/POST/PUT/DELETE | 定时任务管理（cron） |
| `/api/v1/synonyms` | GET/POST/PUT/DELETE | 同义词表管理 |
| `/api/v1/sql/test` | POST | 试跑 SQL（**SELECT 白名单**，LIMIT 20 预览） |
| `/api/v1/metrics` | GET | Prometheus 格式（预留，实现后置） |
| `/api/v1/users` | GET/POST | 用户管理（admin） |
| `/api/v1/auth/login` | POST | 登录签发 JWT |

## 安全

- JWT Bearer（`security.jwt_secret`，过期 24h 可配）；admin/viewer 两级角色
- 试跑 SQL 仅 SELECT 白名单（防注入/写操作）
- 全链路超时：Zinc 查询 1s，超时返回 503
- 内网部署 + 防火墙说明（README）
