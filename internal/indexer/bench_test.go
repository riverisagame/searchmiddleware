package indexer

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/glebarez/sqlite"

	"searchmiddleware/internal/config"
)

// 文档构建器全量扫描吞吐（1000 行 + attrs 合并）
func BenchmarkBuilderScan1000(b *testing.B) {
	db, err := sql.Open("sqlite", "file:bench_idx?mode=memory&cache=shared")
	if err != nil {
		b.Fatal(err)
	}
	// 至少 2 连接：主查询 rows 持有连接期间 attrs 查询需要第二条（1 连接会死锁，同生产 max_open=1 边界）
	db.SetMaxOpenConns(2)
	// -cpu=N 并行 benchmark 会并发 setup：幂等建表/插入
	if _, err := db.Exec("CREATE TABLE IF NOT EXISTS items (id INTEGER PRIMARY KEY, name TEXT, site_id INTEGER, price REAL, tag TEXT)"); err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 1000; i++ {
		if _, err := db.Exec("INSERT OR REPLACE INTO items VALUES (?, ?, ?, ?, ?)", i, fmt.Sprintf("商品%d号", i), i%5, float64(i)*1.5, fmt.Sprintf("tag%d", i%10)); err != nil {
			b.Fatal(err)
		}
	}
	indexCfg := &config.IndexConfig{
		Source: config.IndexSourceConfig{
			Name:       "items",
			DataSource: "d1",
			SQLQuery:   "select id, name, site_id, price from items",
			SQLJoinedField: map[string]string{
				"attrs": "select id, tag from items",
			},
		},
		Index: config.IndexIndexConfig{
			Fields: map[string]config.FieldConfig{
				"id":       {Type: "numeric"},
				"name":     {Type: "text"},
				"site_id":  {Type: "numeric"},
				"price":    {Type: "numeric"},
				"attrs":    {Type: "keyword"},
			},
		},
	}
	builder := NewDocumentBuilder(indexCfg, db)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res, err := builder.BuildFull(context.Background())
		if err != nil {
			b.Fatal(err)
		}
		if res.Count != 1000 {
			b.Fatalf("count=%d", res.Count)
		}
	}
}
