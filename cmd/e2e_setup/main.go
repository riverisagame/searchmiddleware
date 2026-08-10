// 建测试库 + 表 + 数据（匹配 maintenance.yaml 结构）
package main

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	db, err := sql.Open("mysql", "root:123456@tcp(127.0.0.1:3306)/")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	stmts := []string{
		"DROP DATABASE IF EXISTS sm_e2e",
		"CREATE DATABASE sm_e2e CHARACTER SET utf8mb4",
		"USE sm_e2e",
		`CREATE TABLE shop_maintenance (
			maintenance_id BIGINT PRIMARY KEY,
			site_id BIGINT,
			maintenance_name VARCHAR(255),
			sub_title VARCHAR(255),
			status TINYINT,
			is_show TINYINT,
			sort INT,
			price DECIMAL(10,2),
			update_time BIGINT,
			delete_time BIGINT DEFAULT 0
		)`,
		`CREATE TABLE shop_maintenance_category (
			category_id BIGINT PRIMARY KEY,
			category_name VARCHAR(255),
			is_show TINYINT DEFAULT 1
		)`,
		`CREATE TABLE shop_maintenance_category_relation (
			maintenance_id BIGINT,
			category_id BIGINT,
			delete_time BIGINT DEFAULT 0,
			PRIMARY KEY (maintenance_id, category_id)
		)`,
		// 3 条服务数据（update_time 递增，用于增量游标）
		"INSERT INTO shop_maintenance VALUES (1, 1, '发动机维修', '引擎故障诊断', 1, 1, 1, 200.00, 1000, 0)",
		"INSERT INTO shop_maintenance VALUES (2, 1, '轮胎更换', '四轮定位', 1, 1, 2, 150.00, 2000, 0)",
		"INSERT INTO shop_maintenance VALUES (3, 2, '空调保养', '制冷剂加注', 1, 1, 3, 300.00, 3000, 0)",
		"INSERT INTO shop_maintenance_category VALUES (10, '发动机', 1)",
		"INSERT INTO shop_maintenance_category VALUES (20, '轮胎', 1)",
		"INSERT INTO shop_maintenance_category VALUES (30, '空调', 1)",
		"INSERT INTO shop_maintenance_category_relation VALUES (1, 10, 0)",
		"INSERT INTO shop_maintenance_category_relation VALUES (2, 20, 0)",
		"INSERT INTO shop_maintenance_category_relation VALUES (3, 30, 0)",
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			panic(fmt.Sprintf("%s\n  ERR: %v", s, err))
		}
	}
	fmt.Println("sm_e2e DB ready: 3 maintenance + 3 categories + 3 relations")

	// 验证
	var n int
	db.QueryRow("SELECT COUNT(*) FROM sm_e2e.shop_maintenance").Scan(&n)
	fmt.Printf("rows in shop_maintenance: %d\n", n)
}
