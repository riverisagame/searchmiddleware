// 清理测试残留调度（conc_idx_*）并检查 sm_test 配置
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const base = "http://localhost:8090"

func main() {
	login, _ := json.Marshal(map[string]string{"username": "admin", "password": "admin123"})
	req, _ := http.NewRequest("POST", base+"/api/v1/auth/login", bytes.NewReader(login))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	var lr map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&lr)
	resp.Body.Close()
	t := lr["data"].(map[string]interface{})["token"].(string)

	// 列出调度
	req2, _ := http.NewRequest("GET", base+"/api/v1/schedules", nil)
	req2.Header.Set("Authorization", "Bearer "+t)
	resp2, _ := http.DefaultClient.Do(req2)
	b, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	var r map[string]interface{}
	json.Unmarshal(b, &r)
	del := 0
	if data, ok := r["data"].([]interface{}); ok {
		for _, s := range data {
			m := s.(map[string]interface{})
			name := fmt.Sprintf("%v", m["index_name"])
			if strings.Contains(name, "conc_idx") || strings.Contains(name, "audit") {
				id := fmt.Sprintf("%v", m["id"])
				req3, _ := http.NewRequest("DELETE", base+"/api/v1/schedules/"+id, nil)
				req3.Header.Set("Authorization", "Bearer "+t)
				resp3, _ := http.DefaultClient.Do(req3)
				io.Copy(io.Discard, resp3.Body)
				resp3.Body.Close()
				fmt.Printf("deleted schedule id=%s (%s) -> %d\n", id, name, resp3.StatusCode)
				del++
			}
		}
	}
	fmt.Println("deleted:", del)
}
