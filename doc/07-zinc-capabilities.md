# ZincSearch++（车鲸鱼定制版）能力调研

> 日期：2026-08-05 | 源码：D:\claudeprj\zincsearch（module: github.com/riverisagame/zincsearchplusplus, go 1.26）

## 结论速览

车鲸鱼自有定制版 Zinc（riverisagame org）——**中文能力远超官方原版**：jieba 多模式分词 + **内置拼音** + 用户词典热加载。搜索中间件直接受益，拼音零开发。

## 分词器/分析器能力（pkg/analysis 实测）

| 分析器 | 说明 |
|--------|------|
| `NewJiebaStandardAnalyzer` | jieba 标准模式（索引用） |
| `NewJiebaSearchAnalyzer` | jieba 搜索模式（查询用） |
| `NewJiebaFullAnalyzer` | jieba 全模式 |
| `NewJiebaStandardNoHMMAnalyzer` / `NewJiebaSearchNoHMMAnalyzer` | 无 HMM 变体 |
| `NewGseStandardAnalyzer` / `NewGseSearchAnalyzer` | gse 备用后端（DAG+HMM，词典 ~20 万词，准确率低于 jieba） |
| `NewGseStopTokenFilter` | 停用词过滤 |

## 拼音能力（内置，零开发）✅

config.go:119-138：
- `PinyinPath` —— 拼音词典路径
- `PinyinFull` —— 全拼扩展（"手机"→"shouji"）
- `PinyinFirstLetter` —— 首字母扩展（"手机"→"sj"）
- `PinyinKeepOriginal` —— 扩展后保留原词

环境变量（compat_test.go / config_test.go 实证）：
```
ZINC_ANALYSIS_PINYIN_FULL=true            # 强制全拼
ZINC_ANALYSIS_PINYIN_FIRST_LETTER=true    # 首字母（推断命名，config_test 实证 PinyinFirstLetter）
ZINC_ANALYSIS_PINYIN_KEEP_ORIGINAL=true   # 保留原词
```

## 用户词典（热加载）✅

- `ZINC_ANALYSIS_USER_DICT` 环境变量指定词典文件路径
- `backend_jieba_integration_test.go` 实证：UserDictFile / EnvUserDict / DictKeyWords 加载
- `backend_jieba_reload_test.go`：`ReloadJiebaBackend` 支持**运行时热重载**（词典更新无需重启）

## 其他 env 配置（compat_test 实证）

```
ZINC_ANALYSIS_BACKEND=jieba       # 后端切换（jieba/gse）
ZINC_ANALYSIS_CONCURRENCY=8       # 并发
ZINC_ANALYSIS_CACHE_SIZE=50000    # token 缓存
ZINC_ANALYSIS_DEBUG=true          # 调试日志
```

## 缺失能力（影响方案）

| 能力 | 状态 | 对策 |
|------|------|------|
| **synonym filter** | ❌ 无（全源码无 synonym 实现） | 同义词 = 查询期扩展（QueryBuilder + GUI 同义词表） |
| 拼音分析器 | ✅ **内置**（env 开启） | 零开发，docker-compose 配置 env |
| 自定义 analyzer 注册 | 内置固定集合（jieba/gse 系列） | schema 的 analyzers 引用内置集合；mapping 透传兜底 |

## 对搜索中间件的影响

1. **拼音**：不需要同步期生成拼音字段/go-pinyin——Zinc 分析器层直接支持（容器 env 开启）
2. **分词**：schema analyzer 可选 jieba_standard/jieba_search/jieba_full/gse 等
3. **词典**：diy.txt 挂载 USER_DICT + 热重载（词典更新即时生效）
4. **同义词**：确认查询期扩展方案（Zinc 层无此能力）
5. **部署**：docker-compose 使用定制版镜像（riverisagame/zincsearchplusplus 或本地构建）
