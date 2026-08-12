// 诊断 full sync 500
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func main() {
	login, _ := json.Marshal(map[string]string{"username": "admin", "password": "admin123"})
	req, _ := http.NewRequest("POST", "http://localhost:8090/api/v1/auth/login", bytes.NewReader(login))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	var lr map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&lr)
	resp.Body.Close()
	t := lr["data"].(map[string]interface{})["token"].(string)

	req2, _ := http.NewRequest("POST", "http://localhost:8090/api/v1/indexes/maintenance/sync",
		strings.NewReader(`{"type":"full"}`))
	req2.Header.Set("Authorization", "Bearer "+t)
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req2)
	if err != nil {
		fmt.Println("ERR:", err)
		return
	}
	b, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	fmt.Printf("code=%d body=%s\n", resp2.StatusCode, string(b))
}
