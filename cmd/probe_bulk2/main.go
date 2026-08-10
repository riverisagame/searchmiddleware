// 直接测 zinc.Client.Bulk 对新 write 索引
package main

import (
	"fmt"
	"time"

	"searchmiddleware/internal/config"
	"searchmiddleware/internal/zinc"
)

func main() {
	client := zinc.NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {"http://localhost:4081"}},
		Default:  "default",
		Username: "admin",
		Password: "Complexpass#123",
	})

	idx := "dev_maintenance_write_1786334120004"
	docs := []map[string]interface{}{
		{"_id": "1", "maintenance_name": "发动机维修", "price": 200},
	}
	start := time.Now()
	err := client.Bulk(idx, docs, "default")
	fmt.Printf("bulk err=%v took=%v\n", err, time.Since(start))
	if err != nil {
		return
	}
	// 查
	resp, err := client.Search(idx, map[string]interface{}{
		"size": 0, "query": map[string]interface{}{"match_all": map[string]interface{}{}},
	}, "default")
	if err != nil {
		fmt.Println("search err:", err)
		return
	}
	hits, _ := resp["hits"].(map[string]interface{})
	total, _ := hits["total"].(map[string]interface{})
	fmt.Printf("after direct bulk: total=%v\n", total["value"])

	// 清理
	client.DeleteDoc(idx, "1", "default")
}
