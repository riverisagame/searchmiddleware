// 查 sync_logs 的 WARN/failed 记录（run_id=2013 附近）
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
	type Log struct {
		RunID   uint
		Level   string
		Message string
	}
	var logs []Log
	db.Raw("SELECT run_id, level, message FROM sync_logs WHERE level = 'WARN' OR run_id = 2013 ORDER BY id DESC LIMIT 6").Scan(&logs)
	for _, l := range logs {
		m := l.Message
		if len(m) > 300 {
			m = m[:300]
		}
		fmt.Printf("run=%d lvl=%s msg=%q\n", l.RunID, l.Level, m)
	}
}
