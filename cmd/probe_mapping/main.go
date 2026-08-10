// 查索引 mapping + bulk 响应详情
package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
)

func main() {
	zinc := "http://localhost:4081"
	idx := "dev_maintenance_write_1786334735003"

	// 1. mapping
	req, _ := http.NewRequest("GET", zinc+"/es/"+idx+"/_mapping", nil)
	req.SetBasicAuth("admin", "Complexpass#123")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("mapping ERR:", err)
		return
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Println("[mapping]", string(b))

	// 2. bulk 原始响应
	ndjson := "{\"index\":{\"_id\":\"1\"}}\n{\"maintenance_name\":\"\u53d1\u52a8\u673a\u7ef4\u4fee\",\"price\":200}\n"
	req2, _ := http.NewRequest("POST", zinc+"/es/"+idx+"/_bulk?refresh=true", bytes.NewReader([]byte(ndjson)))
	req2.SetBasicAuth("admin", "Complexpass#123")
	req2.Header.Set("Content-Type", "application/x-ndjson")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		fmt.Println("bulk ERR:", err)
		return
	}
	b2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	fmt.Println("[bulk resp]", string(b2))

	// 3. 再查
	req3, _ := http.NewRequest("POST", zinc+"/es/"+idx+"/_search", bytes.NewReader([]byte(`{"size":0,"query":{"match_all":{}}}`)))
	req3.SetBasicAuth("admin", "Complexpass#123")
	req3.Header.Set("Content-Type", "application/json")
	resp3, _ := http.DefaultClient.Do(req3)
	b3, _ := io.ReadAll(resp3.Body)
	resp3.Body.Close()
	fmt.Println("[search]", string(b3))
}
