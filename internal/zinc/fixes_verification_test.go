package zinc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"searchmiddleware/internal/config"
)

// TestRealZinc_BoostHotReload 验证 BUG-002 修复
// 注意：Bluge 的 boost 是索引期参数。热更新 mapping 只影响新索引的文档。
// 所以验证两点：(1) mapping 持久化成功 (2) 新文档使用新 boost
func TestRealZinc_BoostHotReload(t *testing.T) {
	if !pingRealZinc(t) {
		t.Skip("real zinc not available")
	}
	client := NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {"http://localhost:4080"}},
		Default:  "default",
		Username: "admin",
		Password: "Complexpass#123",
	})

	index := fmt.Sprintf("sm_boost_%d", time.Now().UnixNano())
	t.Cleanup(func() { client.DeleteIndex(index, "") })

	// 1. 建索引 boost=1.0，写入 doc1
	if err := client.CreateIndex(index, map[string]interface{}{
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "text", "boost": 1.0, "analyzer": "jieba_std"},
		},
	}, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	doc1 := map[string]interface{}{"_id": "1", "name": "换发动机后胶垫"}
	if err := client.Bulk(index, []map[string]interface{}{doc1}, ""); err != nil {
		t.Fatalf("bulk1: %v", err)
	}
	time.Sleep(1500 * time.Millisecond)
	doc1Score := searchScore(t, client, index, "发动机")
	t.Logf("doc1 score (boost=1.0 at index): %.6f", doc1Score)

	// 2. 热更新 mapping boost → 10.0
	if err := client.UpdateMapping(index, map[string]interface{}{
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "text", "boost": 10.0, "analyzer": "jieba_std"},
		},
	}, ""); err != nil {
		t.Fatalf("update mapping: %v", err)
	}
	mappingOK := verifyMappingBoost(t, client, index)
	t.Logf("mapping boost persisted: %v", mappingOK)

	// 3. 写入 doc2（使用新 boost=10.0）
	doc2 := map[string]interface{}{"_id": "2", "name": "换发动机油封"}
	if err := client.Bulk(index, []map[string]interface{}{doc2}, ""); err != nil {
		t.Fatalf("bulk2: %v", err)
	}
	time.Sleep(1500 * time.Millisecond)
	doc2Score := searchScore(t, client, index, "发动机")
	t.Logf("doc2 score (boost=10.0 at index): %.6f", doc2Score)

	// 判断
	if !mappingOK {
		t.Error("BUG-002: mapping boost update did not persist")
		return
	}
	if doc2Score <= doc1Score {
		t.Errorf("BUG-002 NOT TRULY FIXED: new doc score (%.6f) should be > old doc score (%.6f) after boost change", doc2Score, doc1Score)
	} else {
		t.Logf("BUG-002 PARTIALLY FIXED: mapping persisted, new doc score %.6f > old doc %.6f (%.1fx)", doc2Score, doc1Score, doc2Score/doc1Score)
		t.Log("NOTE: Bluge boost is index-time; existing docs need re-index to benefit. This is a Zinc/Bluge architectural limit, not a mapping-update bug.")
	}
}

// TestRealZinc_SynonymReload 验证 REQ-002
func TestRealZinc_SynonymReload(t *testing.T) {
	if !pingRealZinc(t) {
		t.Skip("real zinc not available")
	}
	client := NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {"http://localhost:4080"}},
		Default:  "default",
		Username: "admin",
		Password: "Complexpass#123",
	})

	if err := client.ReloadSynonym(""); err != nil {
		t.Errorf("REQ-002 NOT FIXED: %v", err)
		return
	}
	t.Log("REQ-002 FIXED: POST /api/_reload/synonym succeeded")
}

func searchScore(t *testing.T, c *Client, index, keyword string) float64 {
	t.Helper()
	resp, err := c.Search(index, map[string]interface{}{
		"query": map[string]interface{}{"match": map[string]interface{}{"name": keyword}},
		"size":  2,
	}, "")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	hits, ok := resp["hits"].(map[string]interface{})
	if !ok {
		return 0
	}
	arr, ok := hits["hits"].([]interface{})
	if !ok || len(arr) == 0 {
		return 0
	}
	return arr[0].(map[string]interface{})["_score"].(float64)
}

func verifyMappingBoost(t *testing.T, c *Client, index string) bool {
	t.Helper()
	url, _ := c.getHealthyURL("")
	req, _ := http.NewRequest("GET", url+"/api/"+index+"/_mapping", nil)
	req.SetBasicAuth("admin", "Complexpass#123")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	t.Logf("GET mapping raw: %s", truncate(string(body), 500))
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	// 响应结构：{index_name: {mappings: {properties: {name: {boost: N}}}}}
	var inner map[string]interface{}
	for _, v := range result {
		inner = v.(map[string]interface{})
		break
	}
	mappings, _ := inner["mappings"].(map[string]interface{})
	props, _ := mappings["properties"].(map[string]interface{})
	name, _ := props["name"].(map[string]interface{})
	boost, _ := name["boost"].(float64)
	return boost == 10.0
}
