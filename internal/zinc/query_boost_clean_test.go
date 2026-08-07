package zinc

import (
	"fmt"
	"testing"
	"time"

	"searchmiddleware/internal/config"
)

// TestRealZinc_QueryLevelBoost_Clean 干净隔离验证查询期 field boost
// doc1 只在 name 命中（name^5），doc2 只在 content 命中（content^1）
// 若 boost 生效，doc1 分数应显著高于 doc2
func TestRealZinc_QueryLevelBoost_Clean(t *testing.T) {
	if !pingRealZinc(t) {
		t.Skip("real zinc not available")
	}
	client := NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {"http://localhost:4080"}},
		Default:  "default",
		Username: "admin",
		Password: "Complexpass#123",
	})

	index := fmt.Sprintf("sm_qb_%d", time.Now().UnixNano())
	t.Cleanup(func() { client.DeleteIndex(index, "") })

	if err := client.CreateIndex(index, map[string]interface{}{
		"properties": map[string]interface{}{
			"name":    map[string]interface{}{"type": "text", "analyzer": "jieba_std"},
			"content": map[string]interface{}{"type": "text", "analyzer": "jieba_std"},
		},
	}, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	// doc1 只有 name 含关键词；doc2 只有 content 含关键词
	client.Bulk(index, []map[string]interface{}{
		{"_id": "1", "name": "发动机维修", "content": "汽车保养服务项目介绍"},
		{"_id": "2", "name": "轮胎更换", "content": "发动机皮带定期检查知识"},
	}, "")
	time.Sleep(1500 * time.Millisecond)

	// 1. 无 boost 基线（name 和 content 平等）
	resp0, err := client.Search(index, map[string]interface{}{
		"query": map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  "发动机",
				"fields": []string{"name", "content"},
				"type":   "best_fields",
			},
		},
		"size": 2,
	}, "")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	hits0 := resp0["hits"].(map[string]interface{})["hits"].([]interface{})
	for _, h := range hits0 {
		hit := h.(map[string]interface{})
		src := hit["_source"].(map[string]interface{})
		t.Logf("[NO-BOOST] doc %s (%s): %.6f", hit["_id"], src["name"], hit["_score"].(float64))
	}

	// 2. name^5 boost
	resp5, err := client.Search(index, map[string]interface{}{
		"query": map[string]interface{}{
			"multi_match": map[string]interface{}{
				"query":  "发动机",
				"fields": []string{"name^5", "content"},
				"type":   "best_fields",
			},
		},
		"size": 2,
	}, "")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	hits5 := resp5["hits"].(map[string]interface{})["hits"].([]interface{})
	var nameScore, contentScore float64
	for _, h := range hits5 {
		hit := h.(map[string]interface{})
		src := hit["_source"].(map[string]interface{})
		sc := hit["_score"].(float64)
		t.Logf("[NAME^5] doc %s (%s): %.6f", hit["_id"], src["name"], sc)
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

	t.Logf("ratio (name^5 / content): %.2f", nameScore/contentScore)
	if nameScore/contentScore > 2.0 {
		t.Logf("QUERY-LEVEL BOOST WORKS: name^5 = %.4f vs content = %.4f", nameScore, contentScore)
	} else {
		t.Logf("QUERY-LEVEL BOOST DOES NOT WORK: name^5 = %.4f vs content = %.4f (ratio %.2f)", nameScore, contentScore, nameScore/contentScore)
		t.Log("=> Zinc multi_match 解析 fields 的 ^boost 后缀可能被忽略")
	}
}
