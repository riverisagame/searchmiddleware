// 手动 AliasSwap remove 测试
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func main() {
	// remove dev_maintenance from old index
	body := map[string]interface{}{"actions": []map[string]interface{}{
		{"remove": map[string]interface{}{"index": "dev_maintenance_write_1786337040007", "alias": "dev_maintenance"}},
	}}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", "http://localhost:4081/es/_aliases", bytes.NewReader(b))
	req.SetBasicAuth("admin", "Complexpass#123")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("ERR:", err)
		return
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	fmt.Printf("remove code=%d body=%s\n", resp.StatusCode, string(out))

	// 验证 alias
	req2, _ := http.NewRequest("GET", "http://localhost:4081/es/dev_maintenance/_alias", nil)
	req2.SetBasicAuth("admin", "Complexpass#123")
	resp2, _ := http.DefaultClient.Do(req2)
	out2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	fmt.Println("alias now:", string(out2))
}
