// 增量事件构造：修改/新增/软删（测试专用）
package main

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	db, err := sql.Open("mysql", "root:123456@tcp(127.0.0.1:3306)/sm_e2e?charset=utf8mb4&parseTime=true&loc=Local")
	if err != nil {
		panic(err)
	}
	defer db.Close()
	tx, _ := db.Begin()
	// 1. 修改：id=1 名称改 + update_time 推到 cursor 之后
	if _, err := tx.Exec("UPDATE shop_maintenance SET maintenance_name='发动机大修', update_time=9999999998 WHERE maintenance_id=1"); err != nil {
		panic(err)
	}
	// 2. 新增：id=10005
	if _, err := tx.Exec("INSERT INTO shop_maintenance (maintenance_id, site_id, maintenance_name, sub_title, status, is_show, sort, price, update_time, delete_time) VALUES (10005, 1, '变速箱升级测试', '变速系统', 1, 1, 10005, 888.0, 9999999999, 0)"); err != nil {
		panic(err)
	}
	// 3. 软删：id=2 置 delete_time
	if _, err := tx.Exec("UPDATE shop_maintenance SET delete_time=9999999999, update_time=9999999997 WHERE maintenance_id=2"); err != nil {
		panic(err)
	}
	if err := tx.Commit(); err != nil {
		panic(err)
	}
	var cnt int
	db.QueryRow("SELECT COUNT(*) FROM shop_maintenance").Scan(&cnt)
	fmt.Printf("events applied, total rows=%d\n", cnt)
}
