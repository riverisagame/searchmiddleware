// 查 metadata index_configs 残留
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
	type IC struct {
		ID      uint
		Name    string
		Config  string
		Version string
	}
	var rows []IC
	db.Raw("SELECT id, name, config, version FROM index_configs").Scan(&rows)
	for _, r := range rows {
		cfg := r.Config
		if len(cfg) > 80 {
			cfg = cfg[:80]
		}
		fmt.Printf("id=%d name=%s config=%q version=%s\n", r.ID, r.Name, cfg, r.Version)
	}
	fmt.Println("total:", len(rows))
}
