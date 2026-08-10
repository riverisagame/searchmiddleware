// 测 sm 搜索 API + Zinc 连通（带超时）
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func main() {
	// 登录
	login, _ := json.Marshal(map[string]interface{}{"username": "admin", "password": "admin123"})
	req, _ := http.NewRequest("POST", "http://localhost:8090/api/v1/auth/login", bytes.NewReader(login))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("login ERR:", err)
		return
	}
	var lr map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&lr)
	resp.Body.Close()
	token := lr["data"].(map[string]interface{})["token"].(string)

	// sm 搜索
	start := time.Now()
	req2, _ := http.NewRequest("GET", "http://localhost:8090/api/v1/search?index=maintenance&keyword=发动机&limit=5", nil)
	req2.Header.Set("Authorization", "Bearer "+token)
	resp2, err := client.Do(req2)
	if err != nil {
		fmt.Printf("[sm search] ERR after %v: %v\n", time.Since(start), err)
		return
	}
	b2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	fmt.Printf("[sm search] %d in %v: %s\n", resp2.StatusCode, time.Since(start), string(b2)[:300])
}
