// 修复后全面验证：搜索、写入、alias 状态
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func main() {
	client := &http.Client{}
	do := func(method, path, body string) (string, int) {
		var reader io.Reader
		if body != "" {
			reader = strings.NewReader(body)
		}
		req, _ := http.NewRequest(method, "http://localhost:4081"+path, reader)
		req.SetBasicAuth("admin", "Complexpass#123")
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return err.Error(), 0
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return string(b), resp.StatusCode
	}

	// 1. 搜索 alias
	b, code := do("GET", "/es/dev_maintenance/_search?size=10", "")
	var r struct {
		Hits struct {
			Total struct{ Value int `json:"value"` }
		} `json:"hits"`
	}
	json.Unmarshal([]byte(b), &r)
	fmt.Printf("1. search via alias code=%d total=%d\n", code, r.Hits.Total.Value)

	// 2. 写入 alias
	b2, code2 := do("POST", "/es/dev_maintenance/_doc/alias-verify-2", `{"title":"alias-verify-2"}`)
	fmt.Printf("2. write via alias code=%d resp=%s\n", code2, b2)

	// 3. 物理索引 total
	b3, _ := do("GET", "/es/dev_maintenance_write_1786431625032/_search?size=10", "")
	json.Unmarshal([]byte(b3), &r)
	fmt.Printf("3. physical index total=%d\n", r.Hits.Total.Value)
}
