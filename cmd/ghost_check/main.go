// 精确查：幽灵索引与 alias 关系
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
	do := func(path string) (string, int) {
		req, _ := http.NewRequest("GET", "http://localhost:4081"+path, nil)
		req.SetBasicAuth("admin", "Complexpass#123")
		resp, err := client.Do(req)
		if err != nil {
			return "", 0
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return string(b), resp.StatusCode
	}

	// 索引列表（ZINC_INDEX_LIST）
	idxList, _ := do("/api/index")
	hasOld := strings.Contains(idxList, "1786337040007")
	hasNew := strings.Contains(idxList, "1786431625032")
	fmt.Printf("index list: old(7040007)=%v new(1625032)=%v\n", hasOld, hasNew)

	// alias 映射
	al, _ := do("/es/_alias")
	var m map[string]interface{}
	json.Unmarshal([]byte(al), &m)
	for idx := range m {
		if strings.Contains(idx, "maintenance") {
			fmt.Println("alias maps index:", idx)
		}
	}

	// 单索引 GET（存在性）
	b, code := do("/es/dev_maintenance_write_1786337040007/_search")
	_ = b
	fmt.Printf("old index search code=%d\n", code)
}
