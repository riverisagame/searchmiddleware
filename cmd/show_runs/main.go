// 查看同步运行记录
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func main() {
	// 登录
	login, _ := json.Marshal(map[string]interface{}{"username": "admin", "password": "admin123"})
	req, _ := http.NewRequest("POST", "http://localhost:8090/api/v1/auth/login", bytes.NewReader(login))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	var lr map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&lr)
	resp.Body.Close()
	token := lr["data"].(map[string]interface{})["token"].(string)

	// runs
	req2, _ := http.NewRequest("GET", "http://localhost:8090/api/v1/runs", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	resp2, _ := http.DefaultClient.Do(req2)
	b, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	fmt.Println(string(b)[:800])
}
