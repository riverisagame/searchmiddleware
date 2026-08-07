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

// TestRealZinc_DiagMappingBoost 诊断 mapping boost 链路
func TestRealZinc_DiagMappingBoost(t *testing.T) {
	if !pingRealZinc(t) {
		t.Skip("real zinc not available")
	}
	client := NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {"http://localhost:4080"}},
		Default:  "default",
		Username: "admin",
		Password: "Complexpass#123",
	})

	index := fmt.Sprintf("sm_diag2_%d", time.Now().UnixNano())
	t.Cleanup(func() { client.DeleteIndex(index, "") })

	// 1. 创建带 boost 的索引
	if err := client.CreateIndex(index, map[string]interface{}{
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "text", "boost": 10.0, "analyzer": "jieba_std"},
		},
	}, ""); err != nil {
		t.Fatalf("create: %v", err)
	}

	// 2. GET mapping 看 boost 是否在
	url, _ := client.getHealthyURL("")
	req, _ := http.NewRequest("GET", url+"/api/"+index+"/_mapping", nil)
	req.SetBasicAuth("admin", "Complexpass#123")
	resp, err := client.httpClient.Do(req)
	if err != nil {
		t.Fatalf("get mapping: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	t.Logf("GET mapping: %s", truncate(string(body), 300))

	// 3. 解析出 boost 值
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	for _, v := range result {
		inner, _ := v.(map[string]interface{})
		mappings, _ := inner["mappings"].(map[string]interface{})
		props, _ := mappings["properties"].(map[string]interface{})
		name, _ := props["name"].(map[string]interface{})
		t.Logf("mapping name prop keys: %v", keysOf(name))
		boost, ok := name["boost"]
		t.Logf("mapping name.boost = %v (present: %v)", boost, ok)
	}

	// 4. 写文档 + 搜索
	client.Bulk(index, []map[string]interface{}{{"_id": "1", "name": "换发动机后胶垫"}}, "")
	time.Sleep(1500 * time.Millisecond)

	resp2, err := client.Search(index, map[string]interface{}{
		"query": map[string]interface{}{"match": map[string]interface{}{"name": "发动机"}},
	}, "")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	hits, _ := resp2["hits"].(map[string]interface{})["hits"].([]interface{})
	if len(hits) > 0 {
		t.Logf("score with mapping boost=10: %.4f", hits[0].(map[string]interface{})["_score"].(float64))
	} else {
		t.Log("no hits")
	}
}

func keysOf(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
