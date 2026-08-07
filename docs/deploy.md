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
- Zinc `/healthz`：存活探针

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
