# Search Middleware Context

MySQL ↔ ZincSearch 双向索引同步中间件：以 SQL 配置驱动文档组装，提供零停机全量重建、增量游标同步、搜索 API 与 JWT 鉴权。面向车鲸鱼（PHP）搜索业务。

## Language

**全量重建（Full Rebuild）**:
从零重建索引的过程：新建 write 索引 → 全量同步 → 90% 校验门禁 → 别名切换 → 旧索引延迟清理。全程不中断对外搜索。
_Avoid_: 重建、reindex

**增量同步（Incremental Sync）**:
基于 `update_time` 游标 + keyset 分页的定期增量更新，只处理游标之后变更的行。
_Avoid_: 增量更新、delta

**对账（Reconcile）**:
对比 MySQL 有效数据与索引内容的差异检查，分 count 级（数量）与 id 级（ID 差集）两种。
_Avoid_: 校对、consistency check

**差异修复（Diff Fix）**:
对账发现差异后的一次性纠正：缺失的行重建入索引、多余的行从索引删除。
_Avoid_: 补同步

**软删清理（Soft-delete Cleanup）**:
将数据库中 `delete_time != 0`（软删）的行从索引剔除的过程。仅在 id 级对账/每日全量兜底时执行，增量不处理。
_Avoid_: 删除同步

**脏数据窗口（Dirty Window）**:
软删后数据仍残留在索引中可被搜索到的时段。约定上限为一天（由每日全量兜底保证），保底可接受。

**Write 索引（Write Index）**:
当前用于写入数据的物理索引（`<env>_<index>_write_<ts>`）。重建时新建，切换成功后成为只读旧索引。
_Avoid_: 写入索引、主索引

**Read 别名（Read Alias）**:
对外搜索使用的逻辑别名（`<env>_<index>`），始终指向当前有效索引。业务只感知别名，不感知物理索引。
_Avoid_: 搜索别名

**90% 校验门禁（90% Gate）**:
全量重建完成后的正确性校验：索引文档数达到库内有效行数 90% 以上才允许切换别名。
_Avoid_: 完成校验、质量门禁

**配置真相（Config as Source of Truth）**:
索引定义（`config/indexes/*.yaml`）是唯一真相：内容 SHA256 版本标识，热加载，文件与元数据冲突时以文件为准。
_Avoid_: 元数据为准
