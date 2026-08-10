// 真实 builder 测试：BuildFull 对 sm_e2e
package main

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"

	"searchmiddleware/internal/config"
	"searchmiddleware/internal/indexer"
)

func main() {
	db, err := sql.Open("mysql", "root:123456@tcp(127.0.0.1:3306)/sm_e2e?charset=utf8mb4&parseTime=true&loc=Local")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	cfg := &config.IndexConfig{
		Source: config.IndexSourceConfig{
			Name:       "maintenance_source",
			DataSource: "main",
			SQLQuery: `SELECT maintenance_id, site_id, maintenance_name, sub_title,
           status, is_show, sort, price, update_time
    FROM shop_maintenance
    WHERE delete_time = 0`,
			SQLJoinedField: map[string]string{
				"category_names": `SELECT r.maintenance_id, GROUP_CONCAT(c.category_name SEPARATOR ' ') AS val
      FROM shop_maintenance_category_relation r
      JOIN shop_maintenance_category c ON c.category_id = r.category_id
      WHERE r.delete_time = 0 AND c.is_show = 1
      GROUP BY r.maintenance_id`,
			},
			IncrementalField: "update_time",
		},
		Index: config.IndexIndexConfig{
			Fields: map[string]config.FieldConfig{
				"maintenance_name": {Type: "text", Searchable: true},
				"sub_title":        {Type: "text", Searchable: true},
				"category_names":   {Type: "text", Searchable: true},
				"price":            {Type: "float", Sortable: true, Agg: true},
				"site_id":          {Type: "keyword", Filter: true},
				"update_time":      {Type: "date", Format: "unix_timestamp"},
			},
		},
	}

	b := indexer.NewDocumentBuilder(cfg, db)
	res, err := b.BuildFull(context.Background())
	if err != nil {
		fmt.Println("BUILDFULL ERR:", err)
		return
	}
	fmt.Printf("Count=%d LastCursor=%q\n", res.Count, res.LastCursor)
	for _, d := range res.Docs {
		fmt.Printf("  doc: %v\n", d)
	}
}
