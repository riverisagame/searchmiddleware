// 验证 Zinc bulk 写入稳定性（超时探测）
package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"
)

func main() {
	zinc := "http://localhost:4081"
	client := &http.Client{Timeout: 15 * time.Second}

	// 1. 对旧 maintenance 索引 bulk 写 1 条
	ndjson := "{\"index\":{\"_id\":\"probe1\"}}\n{\"name\":\"探测文档\"}\n"
	req, _ := http.NewRequest("POST", zinc+"/es/dev_maintenance_write_1786152495011/_bulk?refresh=true", bytes.NewReader([]byte(ndjson)))
	req.SetBasicAuth("admin", "Complexpass#123")
	req.Header.Set("Content-Type", "application/x-ndjson")
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[bulk old idx] ERR after %v: %v\n", time.Since(start), err)
		return
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Printf("[bulk old idx] %d in %v: %s\n", resp.StatusCode, time.Since(start), string(b)[:min(150, len(b))])

	// 2. 对全新索引 bulk
	req2, _ := http.NewRequest("POST", zinc+"/es/probe_fresh/_bulk?refresh=true", bytes.NewReader([]byte(ndjson)))
	req2.SetBasicAuth("admin", "Complexpass#123")
	req2.Header.Set("Content-Type", "application/x-ndjson")
	start2 := time.Now()
	resp2, err := client.Do(req2)
	if err != nil {
		fmt.Printf("[bulk fresh] ERR after %v: %v\n", time.Since(start2), err)
		return
	}
	b2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	fmt.Printf("[bulk fresh] %d in %v: %s\n", resp2.StatusCode, time.Since(start2), string(b2)[:min(150, len(b2))])

	// 清理
	req3, _ := http.NewRequest("DELETE", zinc+"/api/index/probe_fresh", nil)
	req3.SetBasicAuth("admin", "Complexpass#123")
	client.Do(req3)
	req4, _ := http.NewRequest("DELETE", zinc+"/es/probe_fresh/_doc/probe1", nil)
	req4.SetBasicAuth("admin", "Complexpass#123")
	client.Do(req4)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
