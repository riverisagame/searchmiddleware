package zinc

import (
	"fmt"
	"testing"
	"time"

	"searchmiddleware/internal/config"
)

// TestRealZinc_BoolShouldBoost 验证替代方案：bool should + 多个 match，每个 match 单独 boost
// 这是 searchmiddleware 可用的查询期权重实现（避开 multi_match ^ 语法 bug）
func TestRealZinc_BoolShouldBoost(t *testing.T) {
	if !pingRealZinc(t) {
		t.Skip("real zinc not available")
	}
	client := NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {"http://localhost:4080"}},
		Default:  "default",
		Username: "admin",
		Password: "Complexpass#123",
	})

	index := fmt.Sprintf("sm_boolb_%d", time.Now().UnixNano())
	t.Cleanup(func() { client.DeleteIndex(index, "") })

	if err := client.CreateIndex(index, map[string]interface{}{
		"properties": map[string]interface{}{
			"name":    map[string]interface{}{"type": "text", "analyzer": "jieba_std"},
			"content": map[string]interface{}{"type": "text", "analyzer": "jieba_std"},
		},
	}, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	client.Bulk(index, []map[string]interface{}{
		{"_id": "1", "name": "发动机维修", "content": "汽车保养服务项目介绍"},
		{"_id": "2", "name": "轮胎更换", "content": "发动机皮带定期检查知识"},
	}, "")
	time.Sleep(1500 * time.Millisecond)

	// bool should: name match boost=5, content match boost=1
	resp, err := client.Search(index, map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []interface{}{
					map[string]interface{}{"match": map[string]interface{}{"name": map[string]interface{}{"query": "发动机", "boost": 5.0}}},
					map[string]interface{}{"match": map[string]interface{}{"content": map[string]interface{}{"query": "发动机", "boost": 1.0}}},
				},
			},
		},
		"size": 2,
	}, "")
	if err != nil {
		t.Fatalf("search: %v", err)
	}

	hits := resp["hits"].(map[string]interface{})["hits"].([]interface{})
	var nameScore, contentScore float64
	for _, h := range hits {
		hit := h.(map[string]interface{})
		src := hit["_source"].(map[string]interface{})
		sc := hit["_score"].(float64)
		t.Logf("doc %s (%s): %.6f", hit["_id"], src["name"], sc)
		if hit["_id"] == "1" {
			nameScore = sc
		} else {
			contentScore = sc
		}
	}

	if nameScore == 0 || contentScore == 0 {
		t.Error("missing score")
		return
	}
	ratio := nameScore / contentScore
	t.Logf("ratio (name boost=5 / content boost=1): %.2f", ratio)
	if ratio > 1.5 {
		t.Logf("BOOL-SHOULD BOOST WORKS: %.4f vs %.4f (%.1fx)", nameScore, contentScore, ratio)
	} else {
		t.Errorf("BOOL-SHOULD BOOST DOES NOT WORK: %.4f vs %.4f (ratio %.2f)", nameScore, contentScore, ratio)
	}
}
