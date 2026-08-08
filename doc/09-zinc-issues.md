# ZincSearch++ 需求与 Bug 提报清单（持续更新）

> 提报方：searchmiddleware 项目 | 验证方式：真实 ZincSearch++（本地构建 :4080）+ Go client 集成测试 + 浏览器端到端验证
> 复查日期：2026-08-07 | 结论：**全部 9 项已修复/已闭环（真实 Zinc 回归验证通过）**

## 状态总览

| 编号 | 类型 | 标题 | 严重度 | 状态 | 提交 |
|------|------|------|--------|------|------|
| BUG-001 | Bug | element_type 无效，数值 terms 不命中 | 高 | ✅ 已修复 | `fd0c6a8`+`bb12f48`+`615b35a` |
| REQ-001 | 需求 | range 聚合 ES ranges 数组格式 | 高 | ✅ 已实现 | `8fb2059` |
| SUG-001 | 建议 | Bulk errors 语义 | 中 | ✅ 已修复 | `d3e9395` |
| SUG-002 | 建议 | /health 别名 | 低 | ✅ 已修复 | `dcac095` |
| REQ-002 | 需求 | 同义词重载 HTTP API | 中 | ✅ 已实现 | `05f2fcc` |
| **BUG-002** | **Bug** | **mapping 层 boost 未生效（索引期死字段），查询期 boost 正常** | 中 | ✅ 已修复（方案 B：查询期生效） | `ffad572`+`16ab142` |
| **BUG-003** | **Bug** | **multi_match `field^boost` 语法未解析，字段名错误致文档丢失** | 高 | ✅ 已修复 | `404842f`+`169d108` |
| **BUG-004** | **Bug** | **match fuzziness + 中文 + jieba 分词器交互导致查询静默失效** | 中 | ✅ 已修复 | `169d108` |
| **BUG-005** | **Bug** | **bool 空数组（`filter:[]`/`should:[]`）导致查询静默失效** | 高 | ✅ 已修复+隔离测试 | `404842f`+`169d108` |
| **BUG-006** | **Bug** | **bool.must 嵌套 bool.should 时 boost 应用错乱（方向颠倒）** | 高 | ✅ 已修复 | `169d108` |

### searchmiddleware 规避状态
| Bug | 规避方式 | 验证 |
|-----|----------|------|
| BUG-002 | 不用 mapping 期 boost，改用查询期 field-level boost（bool should + 独立 match） | ✅ 27x 生效 |
| BUG-003 | 不用 multi_match，改用 bool should + 独立 match | ✅ 27x 生效 |
| BUG-004 | 移除 fuzziness 参数（jieba + 同义词 + 拼音已覆盖召回） | ✅ hits 恢复 |
| BUG-005 | 仅在 filter/should 有内容时添加键（不输出空数组） | ✅ hits 恢复 |
| BUG-006 | keyword 查询用顶层 should，不包 must | ✅ 排序正确 |

---

## BUG-002：mapping 层 boost 未生效（索引期死字段），查询期 boost 正常

### 一句话
mapping 里配置的 `properties.name.boost: 5.0` 在**索引期不生效**（代码从未读取），但**查询期通过 JSON 参数传递的 boost 完全正常**（实测 27x）。

### 严重度：中
- 影响：无法通过修改 mapping 配置实现权重热更新（需重建索引）
- 替代方案：查询期 boost 即时生效，无需重建，且更灵活

### 根因（代码级）
`pkg/core/index_shards_document.go` 全文无 `Boost`/`boost` 关键字。`Property.Boost` 仅作为元数据存储，从未传递给 Bluge 的文档索引器。

### 验证数据
| 场景 | boost=1.0 | boost=10.0 | 比值 |
|------|-----------|------------|------|
| 独立索引对比 | 0.261529 | 0.261529 | **1.00x** |
| 同索引热更新后写新 doc | 0.261529 | 0.261529 | **1.00x** |
| GET mapping | `"boost":1` | `"boost":10` | ✅ 元数据持久化 |

### 查询期 boost（替代方案，已验证）
```json
{"bool":{"should":[
  {"match":{"name":{"query":"发动机","boost":5.0}}},
  {"match":{"content":{"query":"发动机","boost":1.0}}}
]}}
```
实测：name 命中文档 6.538，content 命中文档 0.261，**比值 25x**（≈ boost² 平方效应）。

### 期望修复
`index_shards_document.go` 读取 `Property.Boost` 并传递给 Bluge 文档选项，实现真正的索引期 boost 热更新。

---

## BUG-003：multi_match `field^boost` 语法未解析

### 一句话
`"fields": ["name^5", "content"]` 中的 `name^5` 被**原样当作字段名**传给 bluge，导致字段不存在，该子查询静默失效，文档丢失。

### 严重度：高
- 影响：ES 标准字段级 boost 语法在 Zinc 上完全不可用，且表现为"文档静默丢失"（比"权重不生效"更危险）

### 根因（代码级）
`pkg/uquery/query/multi_match.go:171-177`
```go
for _, vvv := range vv {
    vvvStr, _ := zutils.ToString(vvv)
    value.Fields = append(value.Fields, vvvStr)  // ← "name^5" 原样存入，未解析 caret
}
```
后续 `bluge.NewMatchQuery(q).SetField("name^5")` → 字段不存在 → 子查询无匹配。

### 验证数据
| 查询 | 命中数 |
|------|--------|
| `{"multi_match":{"fields":["name","content"],"query":"发动机"}}` | 2 ✅ |
| `{"multi_match":{"fields":["name^5","content"],"query":"发动机"}}` | **1** ❌（name 字段文档丢失） |

### ES 标准
`"fields": ["title^5", "content"]` 是 ES 字段级 boost 标准语法，应解析为字段 `title` + boost `5.0`。

### 期望修复
解析 `field^boost` 后缀：拆出字段名与 boost，per-field 设置 `SetBoost`。

---

## BUG-004：match fuzziness + 中文 + jieba 分词器交互导致查询静默失效

### 一句话
`{"match":{"name":{"query":"发动机","fuzziness":"AUTO"}}}` 对中文查询返回 **0 命中**，根因是 fuzziness 的 AUTO 模式使用**标准分析器**分词，与 jieba 索引分词结果不匹配，编辑距离找不到匹配项。

### 严重度：中
- 影响：所有带 fuzziness 的中文查询静默失效
- 限定条件：英文或 keyword 字段可能正常

### 根因（代码级）
`pkg/uquery/query/fuzzy.go:336` `ParseFuzziness` 函数：
```go
if strings.HasPrefix(val, "AUTO") {
    if zer == nil {
        zer = analyzer.NewStandardAnalyzer()  // ← 标准分析器，非 jieba
    }
    tokens := zer.Analyze([]byte(query))     // ← 中文被当单 token
    // ... 计算编辑距离
}
```
标准分析器对中文 "发动机" 产生单 token，与 jieba 索引的 token 不匹配 → 编辑距离无法命中。

### 验证数据
| 查询 | 命中数 |
|------|--------|
| `{"match":{"name":{"query":"发动机","boost":5}}}` | 2 ✅ |
| `{"match":{"name":{"query":"发动机","boost":5,"fuzziness":"AUTO"}}}` | **0** ❌ |

### 期望修复
fuzziness AUTO 模式应使用字段对应的分析器（而非标准分析器）分词，或跳过 CJK 字符的 fuzziness 计算。

---

## BUG-005：bool 空数组导致查询静默失效

### 一句话
bool query 中输出空的 `filter: []` 或 `should: []` 数组会导致整个查询返回 **0 命中**。

### 严重度：高
- 影响：无 filter/should 条件时生成的查询全部失效（常见场景）

### 根因（代码级）
Zinc 的 bool query 解析器对空数组的处理逻辑异常，导致整个查询被短路。

### 验证数据
| 查询 | 命中数 |
|------|--------|
| `{"bool":{"filter":[],"must":[{...}],"should":[]}}` | **0** ❌ |
| `{"bool":{"must":[{...}]}}` | 2 ✅ |
| `{"bool":{"must":[{"bool":{"should":[...],"minimum_should_match":1}}]}}` | 2 ✅ |

### 期望修复
bool 解析器应忽略空数组（或 searchmiddleware 侧保证不输出空数组键）。

---

## BUG-006：bool.must 嵌套 bool.should 时 boost 应用错乱

### 一句话
当 keyword 查询被包裹在 `bool.must` 内（`{"bool":{"must":[{"bool":{"should":[match name^5, match category^1]}}]}}`）时，boost 应用方向颠倒——低 boost 字段命中文档反而获得更高分数。

### 严重度：高
- 影响：权重排序完全错乱，搜索结果质量严重下降

### 根因（代码级）
Zinc 对嵌套 bool（must 内套 should）的 boost 聚合逻辑异常。可能原因：
- 内层 should 的多个 match boost 被错误聚合（取最大/最小/求和）
- 或 must 层的 boost 传递与 should 层 boost 作用域混淆

### 验证数据
| 查询结构 | doc1(name boost=5) | doc2(category boost=1) | 方向 |
|----------|-------------------|------------------------|------|
| 顶层 should（不嵌套） | 6.538 | 0.261 | ✅ 正确 |
| must 嵌套 should | 0.261 | 6.538 | ❌ 颠倒 |

### 期望修复
修复嵌套 bool 的 boost 传递逻辑，确保 per-field boost 独立作用于对应子查询。

---

## 已修复问题回归基线

| 能力 | 状态 |
|------|------|
| 建索引（element_type/copy_to/sortable/aggregatable） | ✅ |
| Bulk NDJSON（数组字段、中文内容） | ✅ |
| match_all / match（jieba 中文搜索） | ✅ |
| terms filter（数组字段） | ✅ |
| terms / range 聚合 | ✅ |
| numeric 字段排序 | ✅ |
| search_after 分页 | ✅ |
| 单文档删除 | ✅ |
| 别名原子 add/remove（COW 实现） | ✅ |
| `/healthz` + `/health` 健康端点 | ✅ |
| element_type:long 数值查询 | ✅ |
| POST /api/_reload/synonym 同义词重载 | ✅ |
| 查询期 field-level boost（bool should） | ✅ 27x |

---

## 真实 Zinc 回归验证（2026-08-07，最新代码重建）

| 验证项 | 结果 |
|--------|------|
| mapping boost=1 vs 10（独立索引） | 0.26 vs 26.15，**100x** ✅ |
| mapping boost 查询期生效（方案 B） | **100x** ✅ |
| 热更新 boost 后新文档 | 26.15（新）vs 0.26（旧）✅ |
| multi_match name^5 | doc1 命中不丢 + **25x** 权重 ✅ |
| 查询级 boost（bool should） | 19.42 vs 0.17 ✅ |
| fuzziness AUTO 中文 | 不再 0 命中（2 条）✅ |
| CreateIndex / UpdateMapping boost 一致 | 均 26.15 ✅ |

**根因澄清**：此前 HTTP 测试显示 boost 不生效，实为**旧 Zinc 进程**（Zinc 团队在 15:50 后继续提交了 match.go 的 boost 逻辑 ffad572/16ab142，重建并重启后全部生效）。match.go 的 ffectiveBoost = queryBoost × prop.Boost 逻辑与 multi_match 的 per-field SetBoost 均正常。

---

## 新增提报（2026-08-07 第二轮）

### REQ-003：同义词内容级更新 API（P1）

**现状**：POST /api/_reload/synonym 仅触发文件重载（system.go:152 ReloadSynonyms → nalysis.ReloadAllSynonyms()），**不接受请求体内容**。同义词更新必须：外部写文件 → 调重载。

**场景痛点**（searchmiddleware 实测）：
- 当前方案：GUI 改同义词 → searchmiddleware 写共享卷文件（synonyms.txt）→ 调重载
- 多 Zinc 节点（Q14）/容器化部署：每节点需共享同一卷，或逐节点写文件，易错难维护

**期望**：
`
POST /api/_reload/synonym
Content-Type: application/json
{ entries: [[手机, 移动电话], [计算机, 电脑, PC]]}
→ 200 {reloaded: true, entries: 42}
`
内存级替换同义词词典，返回生效条目数。

**影响**：searchmiddleware 同义词管理脱离共享卷依赖；多节点部署一行配置即可。

---

### SUG-003：写入后可见性（refresh）语义（P2）

**现状**：bulk 写入后 ~1s 才可搜索（NRT），无 ?refresh=true|wait_for 请求参数支持（ES 标准）。

**影响**：同步引擎写入后立即读场景（对账、集成测试、GUI 即时验证）需手动等待或显式调 POST /es/:target/_refresh（实测 1.5s 等待）。

**期望**：
- bulk/写入端点支持 ?refresh=true（写入后立即刷新）或 ?refresh=wait_for
- 或文档化默认 refresh 间隔与调优方式

**注**：searchmiddleware 已用 POST /es/:target/_refresh 规避（中间件侧），此条为体验优化建议。

---

## REQ-003 复查（2026-08-07 18:19）

**状态**：⚠️ 部分修复（51e1973 已实现 entries body + refresh 参数）

| 验证项 | 结果 |
|--------|------|
| SUG-003 refresh=true/wait_for | ✅ 已修复：bulk ?refresh=true 立即可见（实测 total=1） |
| REQ-003 entries body（裸启动） | ❌ ntries:0 静默 |
| REQ-003 entries body（配 SYNONYM_PATH + 建索引后） | ✅ ntries:5 生效 |

**根因**：SetSynonymEntries 遍历 synonymProcessors 注册表，注册需 SYNONYM_PATH env + 索引 analyzer 构建两个隐性前置；注册表空时静默返回 0（eloaded:true 误导）。

**完整分析报告已提交**：D:\claudeprj\zincsearch\docs\issues\20260807_req003_partial_fix.md（含建议修复方案 A/B）。

---

## REQ-003 二次复查（2026-08-07 20:55，d1b6c9c）

**状态**：⚠️ 部分修复（缺陷 2 已修复，缺陷 1 保留）

| 验证项 | 结果 |
|--------|------|
| 裸启动 POST entries（无索引） | ntries:0 processors:0 warning: no synonym processor registered: set ZINC_ANALYSIS_SYNONYM_PATH and create an index first — **不再静默** ✅ |
| 缺陷 1（依赖 SYNONYM_PATH + 索引前置） | 仍存在（warning 文本已明确告知） |

**结论**：Zinc 采纳方案 B（诊断字段 + warning），放弃方案 A（懒初始化）。静默误导已消除；功能仍要求配置 env + 先建索引。searchmiddleware 侧：同义词导出仍走共享卷文件 + 重载（现有闭环不受影响），warning 字段可用于 GUI 提示。

---

## 新增提报（2026-08-07 第三轮）

### SEC-001：/api/_reload/synonym 无鉴权（P1 安全）

**现状**：outes.go:200 .POST( /api/_reload/synonym, system.ReloadSynonyms) —— **无 AuthMiddleware**。同一文件其他 API 路由（refresh/mapping/doc/bulk 等）全部带 AuthMiddleware(...)。

**风险**：
- 未认证者可 POST entries **覆盖/清空同义词词典**（内容污染，搜索召回被篡改）
- 可高频触发重载（性能影响）
- 内网部署也不应裸奔

**期望**：与 /api/_refresh 一致加 AuthMiddleware(system.ReloadSynonyms)。

---

### SUG-004：文档删除不支持 refresh 参数（P2）

**现状**：DELETE /es/:target/_doc/:id / /api/:target/_doc/:id 无 ?refresh=true 处理（bulk 已支持，SUG-003 修复过，delete 未覆盖）。

**影响**：同步引擎软删清理后立即对账/查询，删除不可见（NRT 窗口），需手动等或调 _refresh。

**期望**：delete 端点支持 ?refresh=true|wait_for，与 bulk 行为一致。

---

## SEC-001 / SUG-004 复查（2026-08-07 21:55，cf13113）

| 验证项 | 结果 |
|--------|------|
| SEC-001 无鉴权调用 /api/_reload/synonym | **401 拒绝** ✅ |
| SEC-001 带鉴权调用 | 200 正常 ✅ |
| SUG-004 DELETE .../_doc/:id?refresh=true 后立即搜索 | **total=1（删除立即可见）** ✅ |

**结论**：两项均已修复（cf13113），修复方式与建议完全一致（AuthMiddleware + 复用 refreshTarget）。

---

## 新增提报（2026-08-07 第四轮）

### BUG-007：terms 聚合 key 乱码（keyword + element_type 数值字段，P1）

**现象**：category_ids（keyword+element_type:long）terms 聚合返回 key:  \u0001@6p\u0000...（字节乱码），期望 238。

**根因**：ggregation.go:237 只看 prop.Type（keyword→TextValueSource），未考虑 prop.ElementType；写入侧（BUG-001 修复）element_type 数值按 NumericField 存储 → 聚合 TextValueSource 读数值字段返回原始字节 key。

**完整分析**：D:\claudeprj\zincsearch\docs\issues\20260807_terms_agg_key_garbled.md（含最小修复：element_type 数值分支走 NumericValueSource + 验证用例）。

---

## BUG-007 复查（2026-08-08 09:15，8b06228）

| 验证项 | 结果 |
|--------|------|
| element_type:long 字段 terms 聚合 key | **数值 238/239/240**（修复前为字节乱码 \u0001@6p）✅ |
| key_as_string（ES 兼容） | ✅ 同时返回 |

**结论**：已修复（8b06228，含双重根因：value source 选择 + 多值消费）。修复方式与建议一致（element_type 数值 → NumericValueSource）。

---

## 新增提报（2026-08-08 第五轮）

### BUG-008：同义词扩展 token 未参与查询（P0，功能假闭环）

**现象**：analyze 层同义词扩展生效（_analyze 返回含同义词 token），但 match 搜索同义词 0 命中；原词命中正常。

| 查询 | 结果 |
|------|------|
| 原词 手机 | ✅ 1 |
| 同义词 handset | ❌ 0 |
| 同义词 移动电话 | ❌ 0 |

**根因方向**：查询路径（bluge.MatchQuery+SetAnalyzer）与 _analyze（engine 直接分析）脱节；扩展 token 同 position/偏移异常可能被丢弃。

**完整复现**：D:\claudeprj\zincsearch\docs\issues\20260808_synonym_query_miss.md

---

## BUG-008 跟进（2026-08-08 12:00，18c61f3 实测）

**状态**：⚠️ **18c61f3 修复未生效**（最新代码重建实测）

| 场景 | 结果 |
|------|------|
| 原词 手机 | ✅ 1 |
| 同义词 handset（默认） | ❌ 0 |
| 同义词 handset（**显式 jieba_search**） | ❌ **0** |
| _analyze（jieba_search） | ✅ 扩展 token 正常 |

**推断**：显式指定 search analyzer 仍 0 命中 → 根因不在 analyzer 选择（18c61f3 修复点），而在**查询执行层**（bluge.MatchQuery 对同 position 多 token 的消费）。

**跟进报告**：docs/issues/20260808_synonym_query_miss_followup.md（Zinc 仓库）

---

## BUG-008 根因锁定（2026-08-08 15:00，76974d0 构建 + debug 日志实测）

**状态**：✅ **根因确认——HTTP 搜索永不触发 fast path**（非 analyzer 消费问题）

| 项 | 结果 |
|----|------|
| HTTP 搜索 handset（含显式 jieba_search） | ❌ 0 命中 |
| 76974d0 token 诊断日志（fastMatchQuery） | **0 条**（HTTP 从未触发） |
| Zinc 团队单测（直接构造 *meta.Query） | ✅ 命中 |

**源码根因**：
1. `meta/query_dsl.go:119` `ZincQuery.Query interface{}` → HTTP 反序列化为 map
2. `query/query.go:226` fast path 仅处理 `*meta.Query` → map 走普通 MatchQuery（无 synonym 消费）
3. 普通路径无 token 日志（76974d0 仅加在 fastMatchQuery）

**建议（提报 914a337）**：HTTP 入口将 map 转 `*meta.Query` 触发 fast path（方案 A），或普通路径实现 synonym 消费（方案 B）。

**提报文档**：`docs/issues/20260808_synonym_root_cause_http_path.md`（Zinc 仓库，已提交 914a337）

---

## BUG-008 最终结论（2026-08-08 15:20）：真根因 = synonyms.txt BOM 污染（环境问题，Zinc 修复全部有效）

**回归验证（690a3f2 构建 + 无 BOM 词典）**：handset=1、移动电话=1、手机=1 ✅ 全部命中

**真根因**：本机用 PowerShell `Set-Content -Encoding UTF8` 写 synonyms.txt → **带 UTF-8 BOM** → 首个词"手机"被污染为 `\uFEFF手机` → synonym 扩展 token 与索引 token 永不匹配。Zinc 四层测试通过（os.WriteFile 无 BOM）vs 本机失败（BOM）的差异即在于此。

**复盘教训（防复发）**：
1. 写 synonyms.txt 必须 **UTF-8 无 BOM**（可用 Go/编辑器"无 BOM"选项；禁止 PowerShell Set-Content UTF8）
2. 排查 synonym 问题时先查文件头字节（EF BB BF）
3. `_analyze` 中 BOM 不可见，肉眼验证 token 需 hexdump

**修正**：此前"HTTP 不走 fast path 根因"（914a337）是真实架构观察，但非 0 命中根因；普通路径 synonym 消费正常（690a3f2 确认）。

---

## 全量真实场景验证（2026-08-08 15:40，Zinc 690a3f2 + 无 BOM 词典）✅ 全绿

| 场景 | 结果 |
|------|------|
| 同义词（移动电话/handset/原词手机） | ✅ 全命中 |
| 拼音（shouji=1, sj=1） | ✅ |
| 高亮（发动机 → `<mark>`） | ✅ |
| mapping 热更新新字段 | ✅ |
| bool filter 嵌套 | ✅ |
| BUG-007 numeric 聚合 key | ✅ |
| Analyze 扩展（jieba_search 含 手机/handset） | ✅ |

离线 Skip 项（Boost 系/BUG002-004/REQ003/SUG003）不受影响。

---

## 4080 系积压验证全跑（2026-08-08 16:00）✅ 17/17 PASS

Boost 系（QueryLevel/Mapping/HotReload/Diag*/BoolShould）、BUG002/003/004、KeywordArray、REQ003、SUG003 全部通过。

**唯一未完成项**：REQ-003 裸启动（无索引）时 synonym entries API 静默返回 0（建索引后正常 5 条）——**代码级确定性缺陷**，已提报 Zinc（含全局快照修复方案）。
- 提报：`docs/issues/20260808_req003_bare_start_entries.md`（Zinc 仓库）

**已知架构限制**（非缺陷）：BUG-002 Bluge boost 为 index-time，存量文档需 re-index（已记录）。

---

## REQ-003 修复验证（2026-08-08 16:00）✅ 通过（真裸启动黑盒）

**修复状态**：Zinc 工作区已实施（全局快照 + 注册继承 + 空注册表返回展开数），**未提交**（待 Zinc 团队 commit）

| 验证项 | 结果 |
|--------|------|
| 真裸启动（无索引、无 processor）POST entries | ✅ **entries=5**（修复前静默 0） |
| 单测（BareStart/WithRegistered/EmptyClears） | ✅ 3/3 PASS |
| 建索引后新 processor 继承快照（搜"移动电话"） | ✅ 命中 |
| 原词"手机" | ✅ 命中 |

**语义确认**：entries 推送后**替代**文件词典（SetEntries 覆盖语义，handset 文件条目不叠加）——符合"内容级更新"设计；如需合并语义可另行提需求。

**副作用提示**：修复覆盖文件条目（快照优先），已在验证中确认符合预期。

---

## SUG-007/008 提报（2026-08-08 16:10）

- **SUG-007**（性能/可观测性）：HTTP 入口 map Query → `*meta.Query` 触发 fast path（token 日志/短路优化生产路径可用），二次反序列化 + fallback 保兼容
- **SUG-008**（语义）：entries 覆盖文件词典；建议合并（A）或文档化覆盖（B）
- 提报：`docs/issues/20260808_sug007_http_fast_path.md`、`docs/issues/20260808_sug008_entries_merge_semantics.md`（Zinc 仓库）

**Zinc 协作全部收尾**：BUG-002~008、REQ-002/003、SUG-003 全部闭环；SUG-007/008 为后续优化项。

---

## SUG-007/008 决策确认（2026-08-08 16:30）

- **SUG-007**：✅ 接受（ParseQueryDSL 二次反序列化小改）；验收 = HTTP 日志出现 match query tokens
- **SUG-008**：✅ 定为方案 B（覆盖 + 文档化）；当前代码已实现全部语义（entries 会话级替换 / reload 恢复文件 / watch 恢复），仅待 manual 文档化

---

## SUG-007/008 修复验证（2026-08-08 16:40）✅ 全部通过（3ebf357）

### SUG-007（HTTP fast path）
- **HTTP 搜索触发 fast path**：日志出现 "match query tokens after analyzer"（修复前 0 条）✅
- 功能回归：同义词/拼音/高亮/mapping-add 全过 ✅
- Zinc 单测 4/4（fast path 命中、归一化、msm 回退、多键回退）

### SUG-008（覆盖 + 文档化语义）
黑盒语义闭环 ✅：
| 步骤 | 结果 |
|------|------|
| 文件词典 handset | 1 |
| entries 替换后（无 handset） | handset=0、移动电话=1（会话级替换）|
| 无 entries reload | handset=1（恢复文件）|

**Zinc 协作全部闭环**：BUG-002~008、REQ-002/003、SUG-003/007/008 全部修复并验证。

---

## BUG-009 提报（2026-08-08 17:00，P1）：GUI 中文搜索全失败

**实测**：Zinc UI 搜索框（query_string 无 analyzer）中文 0 命中；API 对照 match=1 / query_string=0 / multi_match=0 / query_string name:手机=1。

**根因**：query_string.go:216 无 analyzer 回退 StandardAnalyzer；multi_match.go:284 SetAnalyzer(nil)→bluge standard。中文分词链与 jieba 索引不一致。

**建议**：后端字段级 search analyzer 回退（方案 A）或 UI 发 match/显式 analyzer（方案 B）。
- 提报：`docs/issues/20260808_bug009_query_string_cjk.md`（Zinc 仓库）

---

## BUG-009 部分修复验证（2026-08-08 21:10，6fc34d4）

| 查询 | 结果 |
|------|------|
| query_string 中文裸词（UI 搜索框路径） | ✅ 修复（1） |
| query_string 带字段 / 拼音回归 | ✅ |
| **multi_match 无 fields** | ❌ 仍 0（未覆盖，已跟进提报） |

**状态**：UI 搜索框（query_string 路径）已可用；multi_match 无 fields 待 Zinc 修复（建议回填全字段或参数报错）。
