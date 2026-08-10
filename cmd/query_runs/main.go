// 查 full 运行详情
package main

import (
	"fmt"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func main() {
	db, err := gorm.Open(sqlite.Open(`D:\prj\searchmiddleware\data\metadata.db`), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	type Run struct {
		ID         uint
		IndexName  string
		Type       string
		Status     string
		RowsCount  int64
		DurationMs int64
		Message    string
	}
	var runs []Run
	db.Raw("SELECT id, type, status, rows_count, duration_ms FROM sync_runs WHERE index_name='maintenance' AND type='full' ORDER BY id DESC LIMIT 5").Scan(&runs)
	for _, r := range runs {
		fmt.Printf("id=%d status=%s rows=%d dur=%dms\n", r.ID, r.Status, r.RowsCount, r.DurationMs)
	}
}
