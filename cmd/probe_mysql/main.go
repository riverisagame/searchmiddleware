// 探测本机 MySQL 凭据
package main

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	candidates := []string{
		"root:@tcp(127.0.0.1:3306)/",
		"root:root@tcp(127.0.0.1:3306)/",
		"root:123456@tcp(127.0.0.1:3306)/",
		"root:admin@tcp(127.0.0.1:3306)/",
	}
	for _, dsn := range candidates {
		db, err := sql.Open("mysql", dsn)
		if err != nil {
			fmt.Println("open err:", dsn, err)
			continue
		}
		if err := db.Ping(); err != nil {
			fmt.Println("PING FAIL:", dsn, err)
			db.Close()
			continue
		}
		fmt.Println("PING OK:", dsn)
		db.Close()
	}
}
