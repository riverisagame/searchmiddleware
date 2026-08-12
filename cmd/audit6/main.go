// 第 6 轮：并发竞态 + 修复回归
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const base = "http://localhost:8090"

var fail int

func call(method, path, body, token string) (int, string) {
	client := &http.Client{Timeout: 30 * time.Second}
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
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

func report(name string, ok bool, detail string) {
	s := "PASS"
	if !ok {
		s = "FAIL"
		fail++
	}
	fmt.Printf("[%s] %-38s %s\n", s, name, detail)
}

func main() {
	login, _ := json.Marshal(map[string]string{"username": "admin", "password": "admin123"})
	req, _ := http.NewRequest("POST", base+"/api/v1/auth/login", bytes.NewReader(login))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := http.DefaultClient.Do(req)
	var lr map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&lr)
	resp.Body.Close()
	t := lr["data"].(map[string]interface{})["token"].(string)

	// 1. 并发创建调度（30 个并发，同/不同 index_name）
	var wg sync.WaitGroup
	codes := make([]int, 30)
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("maintenance")
			if i%3 == 0 {
				name = fmt.Sprintf("conc_idx_%d", i%2)
			}
			c, _ := call("POST", "/api/v1/schedules",
				fmt.Sprintf(`{"index_name":%q,"type":"incremental","cron_expr":"*/5 * * * * *"}`, name), t)
			codes[i] = c
		}(i)
	}
	wg.Wait()
	okCount := 0
	for _, c := range codes {
		if c == 200 {
			okCount++
		}
	}
	report("concurrent schedule create (30x)", okCount == 30, fmt.Sprintf("200 x%d", okCount))

	// 2. 并发启停同一调度（竞态：toggle on/off 同时）
	c, b := call("POST", "/api/v1/schedules", `{"index_name":"maintenance","type":"incremental","cron_expr":"*/7 * * * * *"}`, t)
	var sid string
	if c == 200 {
		var r map[string]interface{}
		json.Unmarshal([]byte(b), &r)
		if d, ok := r["data"].(map[string]interface{}); ok {
			sid = fmt.Sprintf("%d", int(d["id"].(float64)))
		}
	}
	report("schedule created for race", sid != "", "id="+sid)
	if sid != "" {
		var wg2 sync.WaitGroup
		raceCodes := make([]int, 10)
		for i := 0; i < 10; i++ {
			wg2.Add(1)
			go func(i int) {
				defer wg2.Done()
				en := i%2 == 0
				c, _ := call("PUT", "/api/v1/schedules/"+sid+"/status", fmt.Sprintf(`{"enabled":%v}`, en), t)
				raceCodes[i] = c
			}(i)
		}
		wg2.Wait()
		allOK := true
		for _, c := range raceCodes {
			if c != 200 {
				allOK = false
			}
		}
		report("concurrent toggle race", allOK, fmt.Sprintf("all 200=%v", allOK))
		// 状态一致性：toggle 后读取无 500
		_, b2 := call("GET", "/api/v1/schedules", "", t)
		report("schedules list after race", strings.Contains(b2, `"id":`+sid), "")
		call("DELETE", "/api/v1/schedules/"+sid, "", t)
	}
	// 清理并发创建的
	_, bl := call("GET", "/api/v1/schedules", "", t)
	for _, line := range strings.Split(bl, "{") {
		if strings.Contains(line, "conc_idx_") {
			var r map[string]interface{}
			json.Unmarshal([]byte("{"+line), &r)
			if id, ok := r["id"].(float64); ok {
				call("DELETE", "/api/v1/schedules/"+fmt.Sprintf("%d", int(id)), "", t)
			}
		}
	}

	// 3. 回归：滚动死循环（对账 id 快速返回，不再挂起）
	start := time.Now()
	c, b = call("POST", "/api/v1/indexes/maintenance/reconcile?type=id", "", t)
	elapsed := time.Since(start)
	report("scroll dead-loop regression", elapsed < 5*time.Second,
		fmt.Sprintf("elapsed=%v code=%d", elapsed.Round(time.Millisecond), c))

	// 4. 回归：ping 连接复用（健康检查连续 10 次无端口耗尽）
	ok := true
	for i := 0; i < 10; i++ {
		c, _ = call("GET", "/health", "", "")
		if c != 200 {
			ok = false
		}
	}
	report("health x10 (conn reuse)", ok, "")

	fmt.Printf("===== FAIL count: %d =====\n", fail)
}
