// Zinc 侧诊断：alias 指向 + 文档 + 搜索
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const zinc = "http://localhost:4081"

func main() {
	client := &http.Client{}
	do := func(method, path string, body []byte) (map[string]interface{}, int) {
		req, _ := http.NewRequest(method, zinc+path, bytes.NewReader(body))
		req.SetBasicAuth("admin", "Complexpass#123")
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			fmt.Println("ERR", path, err)
			return nil, 0
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		var out map[string]interface{}
		json.Unmarshal(b, &out)
		return out, resp.StatusCode
	}

	// 1. alias
	al, _ := do("GET", "/es/_alias", nil)
	for idx := range al {
		fmt.Println("alias index:", idx)
	}

	// 2. maintenance 相关索引
	ml := map[string]interface{}{}
	for idx := range al {
		if bytes.Contains([]byte(idx), []byte("maintenance")) {
			ml[idx] = al[idx]
		}
	}
	// 3. 各 maintenance 索引文档数
	for idx := range ml {
		out, _ := do("POST", "/es/"+idx+"/_search", []byte(`{"size":0,"query":{"match_all":{}}}`))
		if h, ok := out["hits"].(map[string]interface{}); ok {
			if t, ok := h["total"].(map[string]interface{}); ok {
				fmt.Printf("index %s total=%v\n", idx, t["value"])
			}
		}
	}

	// 4. dev_maintenance 搜索
	out, _ := do("POST", "/es/dev_maintenance/_search", []byte(`{"query":{"match":{"maintenance_name":"发动机"}}}`))
	b, _ := json.MarshalIndent(out, "", " ")
	fmt.Println("search:", string(b)[:min(400, len(b))])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
