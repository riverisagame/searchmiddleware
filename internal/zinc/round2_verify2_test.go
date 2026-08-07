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

// TestRealZinc_REQ003_AfterIndexCreate 验证：创建索引（触发 analyzer 构建）后 entries API 才生效
func TestRealZinc_REQ003_AfterIndexCreate(t *testing.T) {
	if !pingRealZinc(t) {
		t.Skip("real zinc not available")
	}
	client := NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {"http://localhost:4080"}},
		Default:  "default",
		Username: "admin",
		Password: "Complexpass#123",
	})

	// 1. 创建索引（trigger analyzer 构建，注册 SynonymProcessor）
	index := fmt.Sprintf("sm_req3_%d", time.Now().UnixNano())
	t.Cleanup(func() { client.DeleteIndex(index, "") })
	if err := client.CreateIndex(index, map[string]interface{}{
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "text", "analyzer": "jieba_std", "search_analyzer": "jieba_search"},
		},
	}, ""); err != nil {
		t.Fatalf("create: %v", err)
	}

	// 2. 调用 entries API
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
	t.Logf("response after index create: %v", result)

	entries, _ := result["entries"].(float64)
	if entries >= 1 {
		t.Logf("REQ-003 PARTIAL: 建索引后 entries API 生效（%v 条），但裸启动（无索引）时静默返回 0", entries)
		t.Log("-> 缺陷：内容级更新应自动创建默认 processor，不依赖索引/analyzer 先存在")
	} else {
		t.Errorf("REQ-003 NOT FIXED: 建索引后 entries 仍为 0")
	}
}
