package zinc

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"searchmiddleware/internal/config"
)

// TestRealZinc_REQ003_SynonymEntriesBody 验证 REQ-003：POST /api/_reload/synonym 支持 body entries
func TestRealZinc_REQ003_SynonymEntriesBody(t *testing.T) {
	if !pingRealZinc(t) {
		t.Skip("real zinc not available")
	}
	client := NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {"http://localhost:4081"}},
		Default:  "default",
		Username: "admin",
		Password: "Complexpass#123",
	})

	url, _ := client.getHealthyURL("")
	body, _ := json.Marshal(map[string]interface{}{
		"entries": [][]string{{"手机", "移动电话"}, {"计算机", "电脑", "PC"}},
	})
	req, _ := http.NewRequest("POST", url+"/api/_reload/synonym", bytes.NewReader(body))
	req.SetBasicAuth("admin", "Complexpass#123")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.httpClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	t.Logf("REQ-003 response: %v", result)
	if resp.StatusCode != 200 {
		t.Errorf("REQ-003 NOT FIXED: status %d", resp.StatusCode)
		return
	}
	entries, ok := result["entries"].(float64)
	if !ok || entries < 1 {
		t.Errorf("REQ-003 response entries missing/zero: %v", result)
		return
	}
	t.Logf("REQ-003 FIXED: entries body accepted, %v entries loaded", entries)
}

// TestRealZinc_SUG003_RefreshParam 验证 SUG-003：bulk ?refresh=true 后立即可见（无需 sleep）
func TestRealZinc_SUG003_RefreshParam(t *testing.T) {
	if !pingRealZinc(t) {
		t.Skip("real zinc not available")
	}
	client := NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {"http://localhost:4081"}},
		Default:  "default",
		Username: "admin",
		Password: "Complexpass#123",
	})

	index := fmt.Sprintf("sm_ref_%d", time.Now().UnixNano())
	t.Cleanup(func() { client.DeleteIndex(index, "") })

	if err := client.CreateIndex(index, map[string]interface{}{
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "text", "analyzer": "jieba_std"},
		},
	}, ""); err != nil {
		t.Fatalf("create: %v", err)
	}

	// 带 refresh=true 的 bulk（模拟 ESBulk 参数）
	url, _ := client.getHealthyURL("")
	ndjson := `{"index":{"_index":"` + index + `","_id":"1"}}` + "\n" +
		`{"name":"换发动机后胶垫"}` + "\n"
	req, _ := http.NewRequest("POST", url+"/es/"+index+"/_bulk?refresh=true", bytes.NewBufferString(ndjson))
	req.SetBasicAuth("admin", "Complexpass#123")
	req.Header.Set("Content-Type", "application/x-ndjson")
	resp, err := client.httpClient.Do(req)
	if err != nil {
		t.Fatalf("bulk: %v", err)
	}
	resp.Body.Close()

	// 立即查询（不 sleep）：refresh=true 应立即可见
	searchResp, err := client.Search(index, map[string]interface{}{
		"query": map[string]interface{}{"match_all": map[string]interface{}{}},
	}, "")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	total := extractTotal(t, searchResp)
	t.Logf("SUG-003 immediate search total (after refresh=true bulk): %d", total)

	if total >= 1 {
		t.Logf("SUG-003 FIXED: ?refresh=true 写入立即可见（无 NRT 等待）")
	} else {
		t.Errorf("SUG-003 NOT FIXED: refresh=true 后立即搜索仍 0 命中")
	}
}

var _ = time.Now
