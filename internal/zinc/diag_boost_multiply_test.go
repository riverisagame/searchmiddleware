package zinc

import (
	"fmt"
	"testing"
	"time"

	"searchmiddleware/internal/config"
)

// TestRealZinc_DiagBoostMultiply 诊断：查询 boost × mapping boost 是否相乘
// 若相乘 → prop.Boost 读取正常，问题在 match.go else-if 分支
// 若不乘 → prop.Boost 在搜索时 = 0
func TestRealZinc_DiagBoostMultiply(t *testing.T) {
	if !pingRealZinc(t) {
		t.Skip("real zinc not available")
	}
	client := NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {"http://localhost:4081"}},
		Default:  "default",
		Username: "admin",
		Password: "Complexpass#123",
	})

	index := fmt.Sprintf("sm_bmult_%d", time.Now().UnixNano())
	t.Cleanup(func() { client.DeleteIndex(index, "") })

	if err := client.CreateIndex(index, map[string]interface{}{
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "text", "boost": 10.0, "analyzer": "jieba_std"},
		},
	}, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	client.Bulk(index, []map[string]interface{}{{"_id": "1", "name": "换发动机后胶垫"}}, "")
	time.Sleep(1500 * time.Millisecond)

	// 场景 A：无查询 boost（mapping boost=10 应生效 → 若生效应 2.6 或 26）
	scoreA := scoreOf(t, client, index, map[string]interface{}{
		"match": map[string]interface{}{"name": "发动机"},
	})
	t.Logf("A. match（mapping boost=10）: %.4f", scoreA)

	// 场景 B：查询 boost=5 + mapping boost=10（若相乘应 5×10 生效）
	scoreB := scoreOf(t, client, index, map[string]interface{}{
		"match": map[string]interface{}{"name": map[string]interface{}{"query": "发动机", "boost": 5.0}},
	})
	t.Logf("B. match boost=5（mapping boost=10）: %.4f", scoreB)

	// 场景 C：无 mapping boost 的对照索引（查询 boost=5 → 25x 基线）
	indexC := fmt.Sprintf("sm_bmult_c_%d", time.Now().UnixNano())
	t.Cleanup(func() { client.DeleteIndex(indexC, "") })
	client.CreateIndex(indexC, map[string]interface{}{
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "text", "analyzer": "jieba_std"},
		},
	}, "")
	client.Bulk(indexC, []map[string]interface{}{{"_id": "1", "name": "换发动机后胶垫"}}, "")
	time.Sleep(1500 * time.Millisecond)
	scoreC := scoreOf(t, client, indexC, map[string]interface{}{
		"match": map[string]interface{}{"name": map[string]interface{}{"query": "发动机", "boost": 5.0}},
	})
	t.Logf("C. 无mapping boost + 查询boost=5: %.4f", scoreC)

	t.Logf("判断: B/A=%.1f (若≈1 则 prop.Boost 未参与) | B/C=%.1f (若≈1 则 mapping boost 完全无效)", scoreB/scoreA, scoreB/scoreC)
}

func scoreOf(t *testing.T, c *Client, index string, q interface{}) float64 {
	t.Helper()
	resp, err := c.Search(index, map[string]interface{}{"query": q}, "")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	hits, _ := resp["hits"].(map[string]interface{})["hits"].([]interface{})
	if len(hits) == 0 {
		return 0
	}
	return hits[0].(map[string]interface{})["_score"].(float64)
}
