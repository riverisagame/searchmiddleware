# 部署与运维手册

## 环境

| 组件 | 端口 | 说明 |
|------|------|------|
| searchmiddleware API | 8090 | 车鲸鱼/外部调用，JWT 鉴权 |
| searchmiddleware GUI | 8091 | Web 管理台（/api 自动代理到 8090） |
| ZincSearch++ | 4080 | 索引/搜索引擎（jieba+拼音+词典） |

## Docker Compose 部署

```bash
# 1. 构建定制版 Zinc 镜像（含 jieba/拼音/词典支持）
cd D:\claudeprj\zincsearch
docker build -t zincsearchplusplus:latest .

# 2. 启动
cd D:\prj\searchmiddleware
docker compose up -d

# 3. 验证
curl http://localhost:8090/health          # {"status":"ok",...}
curl http://localhost:8091/                # Web GUI
```

## 数据卷与备份

| 路径 | 内容 | 备份策略 |
|------|------|----------|
| `zinc-data` | Zinc 索引数据 | 每日增量 + 每周全量 |
| `sm-data` | 元数据库(SQLite) + 同步日志 | 每日快照 |
| `./data/dict` | 词典（只读挂载） | Git 管理 |
| `config/` | 配置文件（唯一真相） | Git 管理 |

## 运维命令

```bash
# 配置预检（部署/CI 前必跑）：全量校验 + 数据源连通 + 索引版本 + Zinc 连通
./searchmiddleware.exe config:check

# 创建管理员
./searchmiddleware.exe user:create admin <密码> admin

# 同义词热更新（GUI 修改后触发 Zinc 重载）
curl -X POST http://localhost:4080/api/_reload/synonym -u admin:Complexpass#123
```

## 升级流程

1. `config:check` 预检（含新配置校验）
2. 备份数据卷
3. 滚动重启 searchmiddleware 容器（配置热加载无需重启，二进制升级需重启）
4. 检查 `/health` + GUI 仪表盘健康度
5. 触发一次增量同步验证链路

## 监控

- `GET /api/v1/metrics`：Prometheus 文本格式（索引文档数/同步任务计数）
- GUI 告警中心：同步失败/对账差异告警
- Zinc `/healthz`：存活探针（compose 已配置 healthcheck）

## 日志采集

| 来源 | 容器 | 格式 | 说明 |
|------|------|------|------|
| API/同步/调度 | sm-middleware | Go 标准库 `log`（stdout 文本行） | `docker logs -f sm-middleware`；无级别过滤，生产接 Loki/ELK 按关键词 |
| Zinc | sm-zinc | zerolog JSON（stdout） | `LOG_LEVEL=debug` 开诊断（含 match query tokens）；生产 info |
| Zinc 搜索审计 | sm-zinc | debug 级 "Search Query Audit" 行 | 调 `LOG_LEVEL=debug` 可查每次搜索的 DSL/耗时/命中数 |

> 注：searchmiddleware 目前使用标准库 log（无日志级别/结构化字段）；如需结构化日志与级别控制，属待实现需求。

## 配置与密钥管理

**配置唯一真相 = YAML 文件**（`-config/-datasources/-indexes` 参数指定路径，无环境变量覆盖机制）：

| 文件 | 内容 | 敏感项 |
|------|------|--------|
| `config/app.yaml` | 服务/安全/同步/Zinc 连接 | `security.jwt_secret`（生产必改，禁止提交 git） |
| `config/datasources.yaml` | 数据源 DSN | 数据库密码（建议 `read_dsn` 只读账号） |
| `config/indexes/*.yaml` | 索引定义（热重载） | — |

生产部署建议：
- `jwt_secret` / DSN 通过 **Docker Secret 或 KMS 生成文件后卷挂载**覆盖默认配置，不入 git
- Zinc 侧凭据由 compose 环境变量注入（`ZINC_USER`/`ZINC_PASSWORD`，Zinc 原生支持 env）
- 如需 searchmiddleware 的 env 覆盖机制（如 `SEARCH_JWT_SECRET`），属待实现需求，可提 issue

## 车鲸鱼接入（PHP）

仅 keyword 有值时走搜索服务；无关键词走原 DB 逻辑（最小侵入）：

```php
// 伪代码：MaintenanceService::getPage keyword 分支
$client = new \GuzzleHttp\Client(['base_uri' => 'http://searchmiddleware:8090', 'timeout' => 0.5]);
try {
    $resp = $client->get('/api/v1/search', [
        'headers' => ['Authorization' => 'Bearer ' . $token],
        'query'   => ['index' => 'maintenance', 'keyword' => $keyword, 'site_id' => $siteId, 'limit' => 20],
    ]);
    $ids = array_column($resp->json()['data']['items'], 'id');
    // 用 $ids 回库查详情组装（展示复用现有 DB 逻辑）
} catch (\Throwable $e) {
    // 降级：记录日志 + 回落 LIKE
    $ids = $db->likeSearch($keyword);
}
```

降级契约：503/429/超时(500ms) → 回落 LIKE。
