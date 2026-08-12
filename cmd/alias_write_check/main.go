// 验证：写入 dev_maintenance alias 时数据落到哪个物理索引
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

	// 写入 alias
	body := `{"title":"alias-write-test-1"}`
	b, code := do("POST", "/es/dev_maintenance/_doc/alias-write-test-1", body)
	fmt.Printf("write via alias code=%d resp=%s\n", code, b)

	// 查新索引
	b2, _ := do("GET", "/es/dev_maintenance_write_1786431625032/_search", "")
	var r struct {
		Hits struct {
			Total struct{ Value int `json:"value"` }
		} `json:"hits"`
	}
	json.Unmarshal([]byte(b2), &r)
	fmt.Printf("new index total=%d\n", r.Hits.Total.Value)

	// 查幽灵索引（应 404）
	b3, code3 := do("GET", "/es/dev_maintenance_write_1786337040007/_search", "")
	fmt.Printf("ghost index code=%d resp=%s\n", code3, truncate(b3, 200))
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
