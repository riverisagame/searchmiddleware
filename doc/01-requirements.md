# 搜索中间件需求总览

> 日期：2026-08-05 | 项目：D:\prj\searchmiddleware（独立 Go 项目）| 状态：需求已对齐（grilling 50 问进行中）

## 一句话定位

**独立的通用索引同步中间件**：一方 MySQL、一方 ZincSearch，中间通过"一系列 SQL 和属性"串联成索引文档，提供搜索 API、定时/手动同步、Web GUI 可视化运维。产品级、可扩展、多索引、多环境。

## 核心需求（用户原话归纳）

| # | 需求 | 细化 |
|---|------|------|
| 1 | 通用索引同步系统 | MySQL ↔ Zinc 双向数据流；SQL 串联属性 |
| 2 | 同步脚本灵活 | 通过一系列 SQL 组装一系列属性（配置驱动，非代码） |
| 3 | Web GUI 可视化 | 定时/手动运行同步、索引管理、配置编辑、日志、对账 |
| 4 | 产品级搜索服务 | 稳定、可运维、可观测、零停机重建、多环境 |
| 5 | 独立项目 | 不寄附于车鲸鱼 PHP 项目（D:\prj\searchmiddleware 独立开发） |
| 6 | 可扩展 | 不同索引（maintenance/goods/...）、不同环境、多数据源、MQ 能力预留 |
| 7 | 第三方优先（已调研） | 无现成方案满足全部需求 → 自研 Go |
| 8 | 配置强壮不错 | 文件唯一真相、原子写、错误隔离、校验链 |

## 技术栈

- Go 1.26（独立二进制 = API + 同步引擎 + 定时调度 + Web GUI embed）
- ZincSearch（**车鲸鱼定制版 zincsearchplusplus**：jieba 五模式分词 + 拼音内置 + 词典热加载）
- Vue 3 + Element Plus（Web GUI，独立前端构建后 embed 进二进制）
- GORM（元数据 ORM，DSN 可配置：SQLite/MySQL）
- MySQL（同步数据源，多数据源注册）
- Docker Compose（部署）

## 关键约束

- 文件配置 = 唯一真相（建索引以配置文件为准，方便持久化）
- 同义词：查询期扩展（Zinc 无 synonym filter）；拼音：Zinc 分析器层配置启用（零开发）
- 车鲸鱼接入：HTTP /search，失败降级 LIKE
