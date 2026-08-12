// 造数：sm_e2e.shop_maintenance N 万行 + 边界行（测试专用，仅 INSERT）
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"math/rand"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var n = flag.Int("n", 10000, "rows to seed")

var names = []string{"发动机", "轮胎", "刹车", "空调", "机油", "变速箱", "避震", "火花塞", "电瓶", "雨刮", "大灯", "排气管", "水箱", "燃油泵", "正时皮带", "离合器", "方向盘", "座椅", "玻璃", "保险杠"}
var subs = []string{"更换", "维修", "保养", "检测", "清洗", "升级", "安装", "调试"}

func main() {
	flag.Parse()
	db, err := sql.Open("mysql", "root:123456@tcp(127.0.0.1:3306)/sm_e2e?charset=utf8mb4&parseTime=true&loc=Local")
	if err != nil {
		panic(err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		panic(err)
	}

	// 确认表结构与当前行数（只读核对）
	var cnt int
	db.QueryRow("SELECT COUNT(*) FROM shop_maintenance").Scan(&cnt)
	fmt.Printf("existing rows: %d\n", cnt)
	if cnt > 0 {
		fmt.Printf("clearing %d test rows (DELETE, keep schema)\n", cnt)
		if _, err := db.Exec("DELETE FROM shop_maintenance"); err != nil {
			panic(err)
		}
	}

	base := time.Now().Unix() - 3600
	rng := rand.New(rand.NewSource(42))
	tx, err := db.Begin()
	if err != nil {
		panic(err)
	}
	stmt, err := tx.Prepare("INSERT INTO shop_maintenance (maintenance_id, site_id, maintenance_name, sub_title, status, is_show, sort, price, update_time, delete_time) VALUES (?,?,?,?,?,?,?,?,?,?)")
	if err != nil {
		panic(err)
	}
	for i := 1; i <= *n; i++ {
		name := names[rng.Intn(len(names))] + names[rng.Intn(len(names))]
		sub := subs[rng.Intn(len(subs))]
		// update_time 每 100 行并列一次（keyset 分页边界）
		ut := base + int64(i/100)*10
		price := float64(rng.Intn(5000)) / 10
		if _, err := stmt.Exec(i, i%3+1, name, sub, i%3+1, 1, i, price, ut, 0); err != nil {
			panic(err)
		}
	}
	// 边界行
	long := ""
	for j := 0; j < 200; j++ {
		long += "长"
	}
	if _, err := stmt.Exec(*n+1, 1, long, "超长字段", 1, 1, *n+1, 99.9, base+100000, 0); err != nil {
		panic(err)
	}
	// 软删行（不应进索引）
	if _, err := stmt.Exec(*n+2, 2, "软删测试项", "不应出现在索引", 2, 1, *n+2, 1.0, base+100001, 9999999999); err != nil {
		panic(err)
	}
	// 并列 update_time 边界（与 id=1 相同）
	if _, err := stmt.Exec(*n+3, 3, "并列时间甲", "边界", 3, 1, *n+3, 2.0, base, 0); err != nil {
		panic(err)
	}
	if _, err := stmt.Exec(*n+4, 3, "并列时间乙", "边界", 3, 1, *n+4, 3.0, base, 0); err != nil {
		panic(err)
	}
	if err := tx.Commit(); err != nil {
		panic(err)
	}
	db.QueryRow("SELECT COUNT(*) FROM shop_maintenance").Scan(&cnt)
	fmt.Printf("seeded, total rows: %d\n", cnt)
}
