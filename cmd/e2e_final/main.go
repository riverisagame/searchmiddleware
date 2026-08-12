// 最终端到端回归：真实 MySQL → Zinc 全链路（当前所有修复后）
package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const base = "http://localhost:8090"
const zinc = "http://localhost:4081"

var fail int

func call(method, path string, body interface{}, token string) (int, string) {
	client := &http.Client{Timeout: 60 * time.Second}
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, base+path, rdr)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return 0, err.Error()
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func zincSearch(q string) float64 {
	body := map[string]interface{}{"query": map[string]interface{}{"match": map[string]interface{}{"maintenance_name": q}}}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", zinc+"/es/dev_maintenance/_search", bytes.NewReader(b))
	req.SetBasicAuth("admin", "Complexpass#123")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return -1
	}
	defer resp.Body.Close()
	var out map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&out)
	if h, ok := out["hits"].(map[string]interface{}); ok {
		if t, ok := h["total"].(map[string]interface{}); ok {
			if v, ok := t["value"].(float64); ok {
				return v
			}
		}
	}
	return -1
}

func report(name string, ok bool, detail string) {
	s := "PASS"
	if !ok {
		s = "FAIL"
		fail++
	}
	fmt.Printf("[%s] %-36s %s\n", s, name, detail)
}

func main() {
	// 登录
	code, body := call("POST", "/api/v1/auth/login", map[string]interface{}{"username": "admin", "password": "admin123"}, "")
	var lr map[string]interface{}
	json.Unmarshal([]byte(body), &lr)
	t := lr["data"].(map[string]interface{})["token"].(string)
	report("login", code == 200, "")

	// 清理旧索引（保证干净重建）
	client := &http.Client{}
	reqD, _ := http.NewRequest("GET", zinc+"/es/_alias", nil)
	reqD.SetBasicAuth("admin", "Complexpass#123")
	respD, _ := client.Do(reqD)
	var al map[string]interface{}
	json.NewDecoder(respD.Body).Decode(&al)
	respD.Body.Close()
	for idx := range al {
		reqX, _ := http.NewRequest("DELETE", zinc+"/api/index/"+idx, nil)
		reqX.SetBasicAuth("admin", "Complexpass#123")
		client.Do(reqX)
	}
	// 清理 metadata 索引记录
	reqM, _ := http.NewRequest("GET", base+"/api/v1/indexes", nil)
	reqM.Header.Set("Authorization", "Bearer "+t)
	// 直接调用后端重建逻辑——通过删除 metadata 文件不可行，改为让全量重建走新索引
	// （cleanup：删除 alias 指向的旧索引即可，metadata 由重建覆盖）

	// 1. 全量同步
	code, body = call("POST", "/api/v1/indexes/maintenance/sync", map[string]interface{}{"type": "full"}, t)
	report("full sync", code == 200, fmt.Sprintf("code=%d %s", code, body[:min(80, len(body))]))
	time.Sleep(3 * time.Second)

	// 2. 搜索验证（jieba 中文 + 别名）
	n1 := zincSearch("发动机")
	report("search 发动机", n1 >= 1, fmt.Sprintf("hits=%v", n1))
	n2 := zincSearch("轮胎")
	report("search 轮胎", n2 >= 1, fmt.Sprintf("hits=%v", n2))
	n3 := zincSearch("空调")
	report("search 空调", n3 >= 1, fmt.Sprintf("hits=%v", n3))

	// 3. 增量：DB 更新 + 新增
	mdb, _ := sql.Open("mysql", "root:123456@tcp(127.0.0.1:3306)/sm_e2e")
	mdb.Exec("UPDATE shop_maintenance SET maintenance_name = '发动机大修', update_time = 9999 WHERE maintenance_id = 1")
	mdb.Exec("INSERT INTO shop_maintenance VALUES (4, 2, '刹车片更换', '制动系统', 1, 1, 4, 120.00, 8888, 0)")
	mdb.Close()
	code, body = call("POST", "/api/v1/indexes/maintenance/sync", map[string]interface{}{"type": "incremental"}, t)
	report("incremental sync", code == 200, fmt.Sprintf("code=%d", code))
	time.Sleep(3 * time.Second)
	n4 := zincSearch("发动机大修")
	report("search 发动机大修 (incremental)", n4 >= 1, fmt.Sprintf("hits=%v", n4))

	// 4. 对账 count
	code, body = call("POST", "/api/v1/indexes/maintenance/reconcile?type=count", nil, t)
	report("reconcile count", code == 200 && stringsContains(body, "index_count"), body[:min(120, len(body))])

	// 5. 对账 id（scroll 修复回归）
	start := time.Now()
	code, body = call("POST", "/api/v1/indexes/maintenance/reconcile?type=id", nil, t)
	report("reconcile id", code == 200 && time.Since(start) < 5*time.Second,
		fmt.Sprintf("code=%d elapsed=%v", code, time.Since(start).Round(time.Millisecond)))

	// 6. 通过搜索 API（middleware 层）
	code, body = call("GET", "/api/v1/search?index=maintenance&keyword=%E5%8F%91%E5%8A%A8%E6%9C%BA", nil, t)
	report("search API", code == 200, fmt.Sprintf("code=%d", code))

	fmt.Printf("===== FAIL count: %d =====\n", fail)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func stringsContains(s, sub string) bool {
	return bytes.Contains([]byte(s), []byte(sub))
}
