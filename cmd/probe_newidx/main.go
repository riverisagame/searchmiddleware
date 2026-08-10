// 复现：新建索引 + 立即 bulk（模拟 sm 全量）
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
	client := &http.Client{Timeout: 15 * time.Second}
	idx := fmt.Sprintf("probe_new_%d", time.Now().UnixMilli())

	// 1. 建索引（带 mapping，模拟 sm CreateWriteIndex）
	mapping := map[string]interface{}{
		"name": map[string]interface{}{
			"mappings": map[string]interface{}{
				"properties": map[string]interface{}{
					"maintenance_name": map[string]interface{}{"type": "text"},
					"price":            map[string]interface{}{"type": "numeric"},
					"update_time":      map[string]interface{}{"type": "date", "format": "unix_timestamp"},
				},
			},
		},
	}
	mb, _ := json.Marshal(mapping)
	req, _ := http.NewRequest("PUT", zinc+"/api/index", bytes.NewReader(mb))
	req.SetBasicAuth("admin", "Complexpass#123")
	req.Header.Set("Content-Type", "application/json")
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[create] ERR after %v: %v\n", time.Since(start), err)
		return
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	fmt.Printf("[create] %d in %v\n", resp.StatusCode, time.Since(start).Round(time.Millisecond))

	// 2. 立即 bulk（sm 的方式：不带 refresh）
	ndjson := "{\"index\":{\"_id\":\"1\"}}\n{\"maintenance_name\":\"\u53d1\u52a8\u673a\",\"price\":200}\n"
	req2, _ := http.NewRequest("POST", zinc+"/es/"+idx+"/_bulk", bytes.NewReader([]byte(ndjson)))
	req2.SetBasicAuth("admin", "Complexpass#123")
	req2.Header.Set("Content-Type", "application/x-ndjson")
	start2 := time.Now()
	resp2, err := client.Do(req2)
	if err != nil {
		fmt.Printf("[bulk] ERR after %v: %v\n", time.Since(start2), err)
	} else {
		b, _ := io.ReadAll(resp2.Body)
		resp2.Body.Close()
		fmt.Printf("[bulk] %d in %v: %s\n", resp2.StatusCode, time.Since(start2).Round(time.Millisecond), string(b)[:150])
	}

	// 3. refresh + 搜索
	req3, _ := http.NewRequest("POST", zinc+"/es/"+idx+"/_refresh", nil)
	req3.SetBasicAuth("admin", "Complexpass#123")
	start3 := time.Now()
	resp3, err := client.Do(req3)
	if err != nil {
		fmt.Printf("[refresh] ERR after %v: %v\n", time.Since(start3), err)
	} else {
		io.Copy(io.Discard, resp3.Body)
		resp3.Body.Close()
		fmt.Printf("[refresh] %d in %v\n", resp3.StatusCode, time.Since(start3).Round(time.Millisecond))
	}

	// 4. 搜索
	sb := []byte(`{"size":0,"query":{"match_all":{}}}`)
	req5, _ := http.NewRequest("POST", zinc+"/es/"+idx+"/_search", bytes.NewReader(sb))
	req5.SetBasicAuth("admin", "Complexpass#123")
	req5.Header.Set("Content-Type", "application/json")
	start5 := time.Now()
	resp5, err := client.Do(req5)
	if err != nil {
		fmt.Printf("[search] ERR after %v: %v\n", time.Since(start5), err)
	} else {
		b5, _ := io.ReadAll(resp5.Body)
		resp5.Body.Close()
		fmt.Printf("[search] %d in %v: %s\n", resp5.StatusCode, time.Since(start5).Round(time.Millisecond), string(b5)[:150])
	}

	// 清理
	req4, _ := http.NewRequest("DELETE", zinc+"/api/index/"+idx, nil)
	req4.SetBasicAuth("admin", "Complexpass#123")
	client.Do(req4)
}
