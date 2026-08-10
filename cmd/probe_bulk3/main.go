// 对当前 write 索引直接 bulk（10s 超时）
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
	idx := "dev_maintenance_write_1786334835004"

	ndjson := "{\"index\":{\"_id\":\"t1\"}}\n{\"maintenance_name\":\"\u6d4b\u8bd5\",\"price\":1}\n"
	req, _ := http.NewRequest("POST", zinc+"/es/"+idx+"/_bulk", bytes.NewReader([]byte(ndjson)))
	req.SetBasicAuth("admin", "Complexpass#123")
	req.Header.Set("Content-Type", "application/x-ndjson")
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[bulk %s] ERR after %v: %v\n", idx, time.Since(start).Round(time.Millisecond), err)
		return
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	fmt.Printf("[bulk %s] %d in %v: %s\n", idx, resp.StatusCode, time.Since(start).Round(time.Millisecond), string(b)[:120])
}
