# 实施路线图（任务清单）

> 日期：2026-08-05 | 绝对手动档：每任务对齐 → TDD → 审核 → commit | 待 Q41-Q50 确认后最终冻结

## 阶段划分

### Phase 1：基础设施（TDD 起点）
| # | 任务 | 内容 | 测试 |
|---|------|------|------|
| 1 | ✅ 骨架 | go.mod、目录、embed 准备 | — |
| 2 | 配置加载 | app.yaml 多环境 + datasources.yaml + indexes/*.yaml 解析（类型校验/默认补全/错误定位） | 单测：解析/校验/默认值/错误行号 |
| 3 | Zinc 客户端 | 多节点轮询/故障切换、ping/search/bulk/alias/index 管理 | 单测：httptest mock（认证/NDJSON/解析） |
| 4 | 元数据（GORM） | users/cursors/runs/logs/schedules/reconcile/alerts/synonyms/index_configs + AutoMigrate + DSN 可配 | 单测：SQLite 内存 CRUD |
| 5 | JWT 鉴权 | 登录签发/校验/角色（admin/viewer） | 单测 |

### Phase 2：同步引擎
| # | 任务 | 内容 | 测试 |
|---|------|------|------|
| 6 | 文档构建器 | base SQL + attrs SQL 合并、keyset 分页、字段类型转换、拼音字段（无需，Zinc 层） | 单测：SQLite 测试库组装 |
| 7 | 增量同步 | 游标读写、互斥锁、跳过、重试（3 次退避）、failed_ids | 单测 |
| 8 | 全量重建 | write 索引 → 90% 校验 → alias 原子切换 → 旧索引延迟删 | 单测（mock Zinc） |
| 9 | notify | 按 id 重建（幂等）| 单测 |
| 10 | 对账+清理 | COUNT 级 + id 级 + 差异补同步（每日合并执行） | 单测 |

### Phase 3：搜索 API + 调度
| # | 任务 | 内容 | 测试 |
|---|------|------|------|
| 11 | QueryBuilder | 权重 boost/同义词扩展/过滤/分页/排序/高亮/聚合透传 | 单测 |
| 12 | /search + /health | 契约实现、错误码、限流、超时 | 单测 + 集成（真实 Zinc 可选） |
| 13 | 定时调度 | cron 全量/增量、并行池、互斥、优雅退出 | 单测 |
| 14 | 指标 | 搜索 QPS/Top 关键词/耗时分布、/metrics 预留 | 单测 |

### Phase 4：Web GUI
| # | 任务 | 内容 |
|---|------|------|
| 15 | GUI 骨架 | Vite+Vue3+EP+Pinia+路由+登录页+布局 |
| 16 | 仪表盘+同步中心 | 索引卡片/健康度/手动触发/运行历史/失败重试 |
| 17 | 定时任务+日志+对账 | 列表/新建/启停/日志筛选/差异补同步 |
| 18 | 配置编辑器 | 索引 CRUD/主 SQL/属性 SQL 试跑/字段探测/权重/高级 |
| 19 | 同义词+告警+用户 | 管理页 |
| 20 | embed 集成 | 前端构建 → Go embed → 单二进制验证 |

### Phase 5：部署与接入
| # | 任务 | 内容 |
|---|------|------|
| 21 | docker-compose | zinc（定制版 env：jieba/拼音/词典）+ searchmiddleware（卷挂载） |
| 22 | config:check + README | 预检命令 + 架构/部署/运维手册 + CHANGELOG |
| 23 | 车鲸鱼接入 | PHP 客户端调 /search + 降级 LIKE + 开关（Q41 确认后细化） |

## 测试策略

- 单元测试：Go 标准 testing + httptest（mock Zinc/DB）+ SQLite 内存（元数据/构建器）
- 集成测试：可选真实 Zinc（本地 docker）+ 测试库 MySQL（事务回滚零残留）
- 性能基准：Zinc 查询/同步吞吐（go test -bench）
- 配置预检：config:check 纳入 CI

## 关键依赖（go.mod）

- github.com/gin-gonic/gin（或标准库 net/http + chi，实现时定）
- gorm.io/gorm + driver（sqlite/mysql）
- gopkg.in/yaml.v3
- github.com/robfig/cron/v3
- github.com/golang-jwt/jwt/v5
- golang.org/x/crypto（bcrypt）
- github.com/fsnotify/fsnotify

## 闭环进度（2026-08-08）

- 任务 12 限流：✅ 已实现（fixed-window per-IP，config-gated，429 挂钩 Q39 指标）
- 任务 14 指标：✅ 已实现（Q39：total/latency histogram/top50/5xx/429）
- 待办：任务 23 车鲸鱼接入（Q41 业务确认）

- 任务 22 CHANGELOG：✅ 已补（v0.1.0，2026-08-10）
