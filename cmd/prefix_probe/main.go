// Zinc prefix query + 桶内分页探测
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func main() {
	client := &http.Client{}
	post := func(body map[string]interface{}) (string, int) {
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "http://localhost:4081/es/dev_maintenance/_search", bytes.NewReader(b))
		req.SetBasicAuth("admin", "Complexpass#123")
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return err.Error(), 0
		}
		defer resp.Body.Close()
		rb, _ := io.ReadAll(resp.Body)
		return string(rb), resp.StatusCode
	}

	// prefix _id="1" 桶
	b, c := post(map[string]interface{}{
		"size": 0,
		"query": map[string]interface{}{
			"prefix": map[string]interface{}{"_id": "1"},
		},
	})
	fmt.Printf("prefix 1: code=%d body=%s\n", c, truncate(b, 200))

	// prefix + from 分页（桶内 >10000 测试：用 "1" 桶 1100 条 <10000 无法触发；用全量 match_all 测 from 11000 验证限制仍在）
	b2, c2 := post(map[string]interface{}{
		"from": 11000, "size": 100,
		"query": map[string]interface{}{"match_all": map[string]interface{}{}},
	})
	fmt.Printf("from 11000: code=%d body=%s\n", c2, truncate(b2, 120))
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
