# 上线前审计报告（Round 1：P0/P1/P2）

日期：2026-08-11
基线：SM master 5de5cd3（含全部修复）、Zinc v0.69.5（65acaa1）、MySQL sm_e2e 10004 行

## 测试范围（grilling 确认）

- P0 数据一致性链路：全量重建→增量→对账→差异修复→别名切换
- P1 并发互斥/重启恢复/安全面
- P2 GUI 冒烟/容器部署演练
- 约束：只修现有功能问题，不加功能；sm_e2e 可写、不 DROP/TRUNCATE；Zinc 问题只上报不修

## 修复清单（SM 侧，全部已提交 + 回归测试）

| # | 严重级 | 问题 | 修复 | 提交 |
|---|--------|------|------|------|
| 1 | P0 | full sync 复用陈旧 WriteIndex（已删索引名）→ 全量写入死索引 | full 总是新建 write 索引 | da50f16 |
| 2 | P0 | bulk refresh=true 触发 Zinc WAL 合并 nil panic（10k 级必崩） | 回归 WAL 路径（Zinc v0.69.5 已含 BUG-012 修复） | da50f16 |
| 3 | P0 | 对账 id 用 search_after 分页（Zinc 失效）→ 只扫 3k 条，假阳性 missing | _id 前缀分桶 + 桶内深分页 | da50f16 |
| 4 | P1 | 对账差异修复 DeleteDoc 走 alias（Zinc 不解析）→ 脏文档删不掉 | 解析 alias→物理索引再删 | da50f16 |
| 5 | P1 | validIndexName 依赖文件系统拒绝 `a:b`（Windows 400 / Linux 200） | 显式拒绝 Windows 保留字符 | da50f16 |
| 6 | P2 | metadata.db 硬编码相对路径（容器部署打不开） | 用 config data_dir | 079042c |
| 7 | P2 | user:create 不支持 --config（容器内建用户路径错） | 支持 --config | 5255acd |
| 8 | P2 | Dockerfile 二进制无可执行权限（容器 Permission denied） | COPY --chmod=0755 | b0fe609 |

## Zinc 上报（不修，等待上游）

- issue #2（P0）：WAL 合并 nil Snapshot panic——refresh=true 批量写入必崩；崩溃残留半写 WAL 导致重启即崩
- issue #3（P1）：search_after 失效 / 无 scroll / result window 硬编码 10000（env 未生效）/ GetDoc·DeleteDoc 不解析 alias

## 验证结果

| 场景 | 结果 |
|------|------|
| 全量重建（10k） | 10003 docs + alias 切换 ✅ |
| 增量三类事件 | 修改/新增/软删脏窗口（Q25 语义）✅ |
| 对账 count/id | 10003=10003；完整扫描精确 extra=[2] ✅ |
| 差异修复 | 软删残留剔除 ✅ |
| 并发互斥 | 并发 full → 409 拒绝 ✅ |
| SM 重启恢复 | 数据/调度/搜索完整 ✅ |
| 安全面 | 401/403/JWT 篡改/viewer 只读 ✅ |
| GUI | 首页 + bundle + 管理 API 全 200 ✅ |
| 容器部署 | 镜像运行 + user:create + 登录 + 搜索（946 命中）✅ |
| go test ./... | 全绿（含新增回归测试）✅ |

## 上线前 Checklist（待办）

1. **Docker Hub secrets**：配置 `DOCKERHUB_USERNAME=dockerdoo` + `DOCKERHUB_TOKEN`，dispatch 重跑即可发布 docker.io
2. **zinc 镜像未发布**：仅本地 Dockerfile——上线需发布（zinc repo 或 compose 本地构建）
3. **生产配置**：`jwt_secret` 必改（当前 change-me-in-production）；DSN 指向生产 MySQL（容器内 127.0.0.1 不通宿主，需 host.docker.internal 或同网络）
4. **孤儿元数据行**（P2）：DELETE 索引不清理 index_configs（sm_test/audit_crud 残留）——不影响功能，待清理
5. **cleanupOldIndexes 限制**（P2）：24h 后仅清 2 个旧索引——多次重建会积累
6. **调度重复**：API 允许创建重复 schedule（历史测试产生 40+ 条）——建议生产限制

## 环境状态

- Zinc v0.69.5（4081，ZINC_MAX_RESULTS 未生效属 Zinc 缺陷）
- SM 修复版（8090/8091，pid 60060）
- MySQL sm_e2e：10004 行（含边界数据）
- 容器演练已清理（sm-deploy-test 已删）
