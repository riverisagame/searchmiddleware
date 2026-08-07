# 索引配置 Schema（Sphinx 风格 + 灵活 v2）

> 日期：2026-08-05 | 配置即代码：新索引 = 复制 YAML 改 SQL/字段/权重，零代码

## 示例：config/indexes/maintenance.yaml

```yaml
# ===== ① source 块：数据怎么来（Sphinx source 语义）=====
source:
  name: maintenance_source
  datasource: main                 # 引用 config/datasources.yaml 的 MySQL 连接
  sql_query: |                     # 主查询（每行 = 文档主字段）
    SELECT maintenance_id, site_id, maintenance_name, sub_title,
           status, is_show, sort, price, update_time
    FROM shop_maintenance
    WHERE delete_time = 0
  # 属性体系（Sphinx sql_attr_* 语义 → Zinc 类型）
  sql_attr_uint:     [ maintenance_id, site_id, status, is_show, sort ]   # 数值：过滤/排序
  sql_attr_float:    [ price ]                                            # 浮点
  sql_attr_timestamp:[ update_time ]                                      # 时间：增量游标+范围过滤
  sql_field_string:  [ sub_title ]                                        # 可过滤可显示
  sql_joined_field:  {                                                    # 属性 SQL（按 id 关联组装）
    category_names: |
      SELECT r.maintenance_id, GROUP_CONCAT(c.category_name SEPARATOR ' ') AS val
      FROM shop_maintenance_category_relation r
      JOIN shop_maintenance_category c ON c.category_id = r.category_id
      WHERE r.delete_time = 0 AND c.is_show = 1
      GROUP BY r.maintenance_id
    sku_names: |
      SELECT maintenance_id, GROUP_CONCAT(sku_name SEPARATOR ' ') AS val
      FROM shop_maintenance_sku
      WHERE delete_time = 0
      GROUP BY maintenance_id
  }
  sql_incremental: "WHERE update_time > :cursor"     # 增量游标（自动与 base WHERE 合并）

# ===== ② index 块：怎么索引/搜索 =====
index:
  name: maintenance
  source: maintenance_source
  alias: true                      # 读写别名（零停机重建）
  datasource: main
  analyzers:                       # 索引级分析器定义（引用/自定义）
    jieba_std:     { type: jieba, search: false }
    jieba_search:  { type: jieba, search: true }
  analyzer: jieba_std               # 索引分词
  search_analyzer: jieba_search     # 查询分词
  min_word_len: 2
  boost:                           # 字段权重（Sphinx field weights）
    maintenance_name: 5.0           # 标题 50%
    sub_title: 3.0                  # 副标题 30%
    category_names: 1.0             # 分类 10%
    sku_names: 1.0                  # SKU 10%
  fields:
    maintenance_name: { type: text, searchable: true }
    sub_title:       { type: text, searchable: true }
    category_names:  { type: text, searchable: true }
    sku_names:       { type: text, searchable: true }
    price:           { type: float, sortable: true, agg: true }
    category_ids:    { type: array, element_type: integer, filter: true, agg: true }
    create_time:     { type: date, format: unix_timestamp }
  mapping_options:                  # Zinc mapping 参数透传兜底
    dynamic: false
    extra: {}
  dictionary:
    path: /data/dict/jieba/user/diy.txt   # 汽修词典（Zinc 热加载）
```

## 类型系统（YAML type → Zinc 类型）

| YAML 类型 | Zinc/Bluge | 说明 |
|-----------|-----------|------|
| integer / long | numeric (int/long) | 整数 |
| float / double | numeric (float/double) | 浮点 |
| bool | boolean | 布尔 |
| date | date | format 可配：unix_timestamp / epoch_millis / Go 布局 |
| keyword | keyword | 精确匹配（不分词） |
| text | text | 分词字段（analyzer 可配） |
| array | array | 元素类型 element_type |
| object | object | 嵌套 |
| geo_point | geo_point | 预留（待验证 Zinc 支持度） |

## 自定义化能力

| 能力 | 机制 |
|------|------|
| 自定义分析器 | `index.analyzers` 区定义（type: jieba/gse/custom + tokenizer + filters），字段级 `analyzer/search_analyzer` 引用 |
| 拼音 | Zinc 层 env（ZINC_ANALYSIS_PINYIN_FULL/FIRST_LETTER/KEEP_ORIGINAL），非 schema |
| 同义词 | 元数据库 synonyms 表（GUI 管理），查询期 QueryBuilder 扩展 |
| mapping 透传 | `mapping_options.extra` 原样合并进 Zinc mapping JSON |
| 词典 | `dictionary.path` 挂载进 Zinc 容器（USER_DICT，热重载） |

## 强壮机制（配置"不错"）

1. **原子写**：写 .tmp → 校验 → rename 覆盖；写前备份 .bak
2. **错误隔离**：单索引文件解析失败只报该文件（GUI 标红），不影响其他索引
3. **错误可读**：报错带"文件:行号 + 原因"（yaml/类型/SQL 连接）
4. **默认值补全**：alias=false、boost=1、min_word_len=2...
5. **YAML 能力**：锚点/合并（`<<`）复用公共片段；示例 config/indexes/_example.yaml
6. **预检命令**：`searchmiddleware config:check`（全量校验 + 试跑 SQL，CI/部署前跑）
7. **热加载**：fsnotify 监听变更 → 重新校验回灌 DB（GUI 实时提示）
8. **字段自动探测**：保存时 `SELECT ... LIMIT 0` 取列名 → GUI 表格预填字段行

## 配置存储同步机制

```
文件 config/indexes/*.yaml = 唯一真相（建索引只读文件）
GUI 保存 → ① 写 DB（展示/编辑态）② 原子写文件（持久化）③ 提示"索引需重建"
服务启动/文件变更 → 文件回灌 DB（GUI 与文件一致）
冲突 → 以文件为准（GUI 提示"文件为准，DB 将被覆盖"）
存储模式 app.yaml `config.storage: file | db` 可配
```
