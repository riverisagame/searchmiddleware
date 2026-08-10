// 对照：带/不带 refresh 的 bulk 对同一索引
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
	idx := "dev_maintenance_write_1786336775009"

	// A. 不带 refresh（sm 方式）
	ndjson := "{\"index\":{\"_id\":\"a1\"}}\n{\"maintenance_name\":\"\u6d4b\u8bd5A\",\"price\":1}\n"
	req, _ := http.NewRequest("POST", zinc+"/es/"+idx+"/_bulk", bytes.NewReader([]byte(ndjson)))
	req.SetBasicAuth("admin", "Complexpass#123")
	req.Header.Set("Content-Type", "application/x-ndjson")
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[A no-refresh] ERR after %v: %v\n", time.Since(start).Round(time.Millisecond), err)
	} else {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Printf("[A no-refresh] %d in %v: %s\n", resp.StatusCode, time.Since(start).Round(time.Millisecond), string(b)[:130])
	}

	// refresh
	reqR, _ := http.NewRequest("POST", zinc+"/es/"+idx+"/_refresh", nil)
	reqR.SetBasicAuth("admin", "Complexpass#123")
	respR, _ := client.Do(reqR)
	io.Copy(io.Discard, respR.Body)
	respR.Body.Close()

	// 查
	check := func(tag string) {
		reqS, _ := http.NewRequest("POST", zinc+"/es/"+idx+"/_search", bytes.NewReader([]byte(`{"size":0,"query":{"match_all":{}}}`)))
		reqS.SetBasicAuth("admin", "Complexpass#123")
		reqS.Header.Set("Content-Type", "application/json")
		respS, _ := client.Do(reqS)
		bS, _ := io.ReadAll(respS.Body)
		respS.Body.Close()
		fmt.Printf("[%s] %s\n", tag, string(bS))
	}
	check("after A + refresh")

	// B. 带 refresh=true
	req2, _ := http.NewRequest("POST", zinc+"/es/"+idx+"/_bulk?refresh=true", bytes.NewReader([]byte(ndjson)))
	req2.SetBasicAuth("admin", "Complexpass#123")
	req2.Header.Set("Content-Type", "application/x-ndjson")
	start2 := time.Now()
	resp2, err := client.Do(req2)
	if err != nil {
		fmt.Printf("[B with-refresh] ERR after %v: %v\n", time.Since(start2).Round(time.Millisecond), err)
	} else {
		b2, _ := io.ReadAll(resp2.Body)
		resp2.Body.Close()
		fmt.Printf("[B with-refresh] %d in %v: %s\n", resp2.StatusCode, time.Since(start2).Round(time.Millisecond), string(b2)[:130])
	}
	check("after B")
}
