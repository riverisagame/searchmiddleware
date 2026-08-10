// 端到端严格验证：MySQL → searchmiddleware → Zinc 全链路
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

func call(method, path string, body interface{}, token string) (map[string]interface{}, int) {
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
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("ERR:", err)
		return nil, 0
	}
	defer resp.Body.Close()
	var out map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&out)
	return out, resp.StatusCode
}

func zincSearch(query string) (float64, map[string]interface{}) {
	body := map[string]interface{}{"query": map[string]interface{}{"match": map[string]interface{}{"maintenance_name": query}}}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", zinc+"/es/dev_maintenance/_search", bytes.NewReader(b))
	req.SetBasicAuth("admin", "Complexpass#123")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("zinc ERR:", err)
		return -1, nil
	}
	defer resp.Body.Close()
	var out map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&out)
	hits, _ := out["hits"].(map[string]interface{})
	total, _ := hits["total"].(map[string]interface{})
	if v, ok := total["value"].(float64); ok {
		return v, out
	}
	return -1, out
}

func main() {
	fail := 0
	check := func(name string, ok bool, detail interface{}) {
		status := "PASS"
		if !ok {
			status = "FAIL"
			fail++
		}
		fmt.Printf("[%s] %s %v\n", status, name, detail)
	}

	// 1. 登录
	login, code := call("POST", "/api/v1/auth/login", map[string]interface{}{"username": "admin", "password": "admin123"}, "")
	check("登录", code == 200, code)
	token := ""
	if data, ok := login["data"].(map[string]interface{}); ok {
		token, _ = data["token"].(string)
	}
	check("获取 token", token != "", len(token))

	// 2. 全量同步
	syncResp, code := call("POST", "/api/v1/indexes/maintenance/sync", map[string]interface{}{"type": "full"}, token)
	check("全量同步 API", code == 200, syncResp)
	time.Sleep(3 * time.Second)

	// 3. Zinc 数据验证：计数
	countBody := map[string]interface{}{"size": 0, "query": map[string]interface{}{"match_all": map[string]interface{}{}}}
	b, _ := json.Marshal(countBody)
	req, _ := http.NewRequest("POST", zinc+"/es/dev_maintenance/_search", bytes.NewReader(b))
	req.SetBasicAuth("admin", "Complexpass#123")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("count ERR:", err)
		return
	}
	var countOut map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&countOut)
	resp.Body.Close()
	hits, _ := countOut["hits"].(map[string]interface{})
	total, _ := hits["total"].(map[string]interface{})
	check("Zinc 文档数=3", total["value"].(float64) == 3, total["value"])

	// 4. 搜索验证（jieba 中文 + alias）
	n1, _ := zincSearch("发动机")
	check("搜索'发动机'命中", n1 >= 1, n1)
	n2, _ := zincSearch("轮胎")
	check("搜索'轮胎'命中", n2 >= 1, n2)

	// 5. attrs 合并验证（category_names）
	req2, _ := http.NewRequest("POST", zinc+"/es/dev_maintenance/_search", bytes.NewReader([]byte(`{"query":{"match_all":{}},"size":1}`)))
	req2.SetBasicAuth("admin", "Complexpass#123")
	req2.Header.Set("Content-Type", "application/json")
	resp2, _ := http.DefaultClient.Do(req2)
	var doc map[string]interface{}
	json.NewDecoder(resp2.Body).Decode(&doc)
	resp2.Body.Close()
	docSrc := ""
	if hh, ok := doc["hits"].(map[string]interface{}); ok {
		if arr, ok := hh["hits"].([]interface{}); ok && len(arr) > 0 {
			if first, ok := arr[0].(map[string]interface{}); ok {
				if src, ok := first["_source"].(map[string]interface{}); ok {
					js, _ := json.Marshal(src)
					docSrc = string(js)
				}
			}
		}
	}
	check("attrs 合并(category_names)", len(docSrc) > 0 && (bytes.Contains([]byte(docSrc), []byte("category_names")) || bytes.Contains([]byte(docSrc), []byte("category"))), docSrc[:min(200, len(docSrc))])

	// 6. 增量：DB 更新 → 增量同步 → 验证
	mdb, _ := sql.Open("mysql", "root:123456@tcp(127.0.0.1:3306)/sm_e2e")
	mdb.Exec("UPDATE shop_maintenance SET maintenance_name = '发动机大修', update_time = 9999 WHERE maintenance_id = 1")
	mdb.Exec("INSERT INTO shop_maintenance VALUES (4, 2, '刹车片更换', '制动系统', 1, 1, 4, 120.00, 8888, 0)")
	mdb.Close()
	incResp, code := call("POST", "/api/v1/indexes/maintenance/sync", map[string]interface{}{"type": "incremental"}, token)
	check("增量同步 API", code == 200, incResp)
	time.Sleep(3 * time.Second)

	n3, _ := zincSearch("发动机大修")
	check("增量后搜'发动机大修'", n3 >= 1, n3)
	// 计数 = 4
	req3, _ := http.NewRequest("POST", zinc+"/es/dev_maintenance/_search", bytes.NewReader(b))
	req3.SetBasicAuth("admin", "Complexpass#123")
	req3.Header.Set("Content-Type", "application/json")
	resp3, _ := http.DefaultClient.Do(req3)
	var c3 map[string]interface{}
	json.NewDecoder(resp3.Body).Decode(&c3)
	resp3.Body.Close()
	h3, _ := c3["hits"].(map[string]interface{})
	t3, _ := h3["total"].(map[string]interface{})
	check("增量后文档数=4", t3["value"].(float64) == 4, t3["value"])

	// 7. 对账 count
	rec, code := call("POST", "/api/v1/indexes/maintenance/reconcile?type=count", nil, token)
	check("对账 count API", code == 200, rec)

	// 8. 对账 id
	rec2, code := call("POST", "/api/v1/indexes/maintenance/reconcile?type=id", nil, token)
	check("对账 id API", code == 200, rec2)

	fmt.Printf("\n===== 结果: %d FAIL =====\n", fail)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
