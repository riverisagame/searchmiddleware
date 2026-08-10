// 模拟 builder 查询：maintenance SQL 对 sm_e2e
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

	rows, err := db.Query(`SELECT maintenance_id, site_id, maintenance_name, sub_title,
           status, is_show, sort, price, update_time
    FROM shop_maintenance
    WHERE delete_time = 0`)
	if err != nil {
		fmt.Println("QUERY ERR:", err)
		return
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var id, site, status, show, sort int64
		var name, sub string
		var price, utime interface{}
		if err := rows.Scan(&id, &site, &name, &sub, &status, &show, &sort, &price, &utime); err != nil {
			fmt.Println("SCAN ERR:", err)
			return
		}
		fmt.Printf("row: id=%d name=%s price=%v(%T) utime=%v(%T)\n", id, name, price, price, utime, utime)
		n++
	}
	fmt.Println("rows:", n)
}
