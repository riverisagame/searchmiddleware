// 探测 Zinc alias 与文档状态
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func main() {
	zinc := "http://localhost:4081"
	client := &http.Client{Timeout: 10 * time.Second}

	do := func(method, path string, body []byte) (map[string]interface{}, error) {
		req, _ := http.NewRequest(method, zinc+path, bytes.NewReader(body))
		req.SetBasicAuth("admin", "Complexpass#123")
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		var out map[string]interface{}
		json.Unmarshal(b, &out)
		return out, nil
	}

	// 1. alias 列表（dev_maintenance 指向哪个索引）
	aliases, err := do("GET", "/es/_alias", nil)
	if err != nil {
		fmt.Println("alias ERR:", err)
		return
	}
	fmt.Println("[alias]")
	for idx, meta := range aliases {
		fmt.Printf("  %s -> %v\n", idx, meta)
	}

	// 2. dev_maintenance 搜索
	count := []byte(`{"size":0,"query":{"match_all":{}}}`)
	out, err := do("POST", "/es/dev_maintenance/_search", count)
	if err != nil {
		fmt.Println("count ERR:", err)
		return
	}
	hits, _ := out["hits"].(map[string]interface{})
	total, _ := hits["total"].(map[string]interface{})
	fmt.Printf("[count] dev_maintenance total=%v\n", total["value"])

	// 3. 直接搜新索引（拿一个索引名）
	for idx := range aliases {
		out2, err := do("POST", "/es/"+idx+"/_search", count)
		if err != nil {
			fmt.Println("ERR", idx, err)
			continue
		}
		h2, _ := out2["hits"].(map[string]interface{})
		t2, _ := h2["total"].(map[string]interface{})
		fmt.Printf("[%s] total=%v\n", idx, t2["value"])
	}
}
