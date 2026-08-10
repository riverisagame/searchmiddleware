// 全新索引：不带 refresh bulk → refresh → 查
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
	client := &http.Client{Timeout: 10 * time.Second}
	idx := fmt.Sprintf("probe_fresh2_%d", time.Now().UnixMilli())

	// 建索引
	mb := []byte(`{"name":{"mappings":{"properties":{"name":{"type":"text"}}}}}`)
	req, _ := http.NewRequest("PUT", zinc+"/api/index", bytes.NewReader(mb))
	req.SetBasicAuth("admin", "Complexpass#123")
	req.Header.Set("Content-Type", "application/json")
	resp, _ := client.Do(req)
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()

	// 不带 refresh bulk
	ndjson := "{\"index\":{\"_id\":\"1\"}}\n{\"name\":\"\u6d4b\u8bd5\"}\n"
	req2, _ := http.NewRequest("POST", zinc+"/es/"+idx+"/_bulk", bytes.NewReader([]byte(ndjson)))
	req2.SetBasicAuth("admin", "Complexpass#123")
	req2.Header.Set("Content-Type", "application/x-ndjson")
	resp2, _ := client.Do(req2)
	io.Copy(io.Discard, resp2.Body)
	resp2.Body.Close()

	// refresh
	req3, _ := http.NewRequest("POST", zinc+"/es/"+idx+"/_refresh", nil)
	req3.SetBasicAuth("admin", "Complexpass#123")
	resp3, _ := client.Do(req3)
	io.Copy(io.Discard, resp3.Body)
	resp3.Body.Close()

	// 查
	req4, _ := http.NewRequest("POST", zinc+"/es/"+idx+"/_search", bytes.NewReader([]byte(`{"size":0,"query":{"match_all":{}}}`)))
	req4.SetBasicAuth("admin", "Complexpass#123")
	req4.Header.Set("Content-Type", "application/json")
	resp4, _ := client.Do(req4)
	b4, _ := io.ReadAll(resp4.Body)
	resp4.Body.Close()
	fmt.Printf("[fresh2 %s] %s\n", idx, string(b4))

	// 清理
	req5, _ := http.NewRequest("DELETE", zinc+"/api/index/"+idx, nil)
	req5.SetBasicAuth("admin", "Complexpass#123")
	client.Do(req5)
}
