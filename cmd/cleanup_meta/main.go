// 清理 metadata 中的 maintenance write 索引记录（恢复一致性）
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
	// 清理 index_configs 与游标（让全量重建全新 write 索引）
	res := db.Exec("DELETE FROM index_configs WHERE name = 'maintenance'")
	fmt.Println("index_configs deleted:", res.RowsAffected, res.Error)
	res2 := db.Exec("DELETE FROM sync_cursors WHERE index_name = 'maintenance'")
	fmt.Println("cursors deleted:", res2.RowsAffected, res2.Error)
}
