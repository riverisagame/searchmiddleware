package zinc

import (
	"fmt"
	"testing"
	"time"

	"searchmiddleware/internal/config"
)

// TestRealZinc_BoostDirectCompare 用两个独立索引直接对比 boost 效果
// 索引 A: boost=1.0, 索引 B: boost=10.0，写入相同文档，比较搜索分数
func TestRealZinc_BoostDirectCompare(t *testing.T) {
	if !pingRealZinc(t) {
		t.Skip("real zinc not available")
	}
	client := NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {"http://localhost:4081"}},
		Default:  "default",
		Username: "admin",
		Password: "Complexpass#123",
	})

	ts := time.Now().UnixNano()
	indexA := fmt.Sprintf("sm_boost_a_%d", ts)
	indexB := fmt.Sprintf("sm_boost_b_%d", ts)
	t.Cleanup(func() {
		client.DeleteIndex(indexA, "")
		client.DeleteIndex(indexB, "")
	})

	doc := map[string]interface{}{"_id": "1", "name": "换发动机后胶垫"}

	// 索引 A: boost=1.0
	if err := client.CreateIndex(indexA, map[string]interface{}{
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "text", "boost": 1.0, "analyzer": "jieba_std"},
		},
	}, ""); err != nil {
		t.Fatalf("create A: %v", err)
	}
	client.Bulk(indexA, []map[string]interface{}{doc}, "")
	time.Sleep(1500 * time.Millisecond)

	// 索引 B: boost=10.0
	if err := client.CreateIndex(indexB, map[string]interface{}{
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "text", "boost": 10.0, "analyzer": "jieba_std"},
		},
	}, ""); err != nil {
		t.Fatalf("create B: %v", err)
	}
	client.Bulk(indexB, []map[string]interface{}{doc}, "")
	time.Sleep(1500 * time.Millisecond)

	scoreA := searchScore(t, client, indexA, "发动机")
	scoreB := searchScore(t, client, indexB, "发动机")

	t.Logf("index A (boost=1.0) score: %.6f", scoreA)
	t.Logf("index B (boost=10.0) score: %.6f", scoreB)

	if scoreA == 0 || scoreB == 0 {
		t.Error("one of the searches returned 0")
		return
	}

	ratio := scoreB / scoreA
	t.Logf("score ratio (B/A): %.2f", ratio)

	if ratio > 5.0 {
		t.Logf("BOOST WORKS at index-time: boost=10 index scores %.1fx higher than boost=1", ratio)
	} else if ratio > 1.5 {
		t.Logf("BOOST PARTIALLY WORKS: ratio %.2f (expected ~10x)", ratio)
	} else {
		t.Logf("BOOST DOES NOT WORK: boost=10 and boost=1.0 have same score (ratio %.2f)", ratio)
		t.Log("=> Zinc/Bluge ignores Property.Boost during indexing")
	}
}

// TestRealZinc_BoostHotReload_SameIndex 热更新场景：同索引改 boost 后重建文档
func TestRealZinc_BoostHotReload_SameIndex(t *testing.T) {
	if !pingRealZinc(t) {
		t.Skip("real zinc not available")
	}
	client := NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {"http://localhost:4081"}},
		Default:  "default",
		Username: "admin",
		Password: "Complexpass#123",
	})

	index := fmt.Sprintf("sm_boost_hot_%d", time.Now().UnixNano())
	t.Cleanup(func() { client.DeleteIndex(index, "") })

	// 1. boost=1.0 建索引 + 写 doc
	if err := client.CreateIndex(index, map[string]interface{}{
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "text", "boost": 1.0, "analyzer": "jieba_std"},
		},
	}, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	client.Bulk(index, []map[string]interface{}{{"_id": "old", "name": "换发动机后胶垫"}}, "")
	time.Sleep(1500 * time.Millisecond)
	oldScore := searchScore(t, client, index, "发动机")
	t.Logf("old doc (boost=1.0): %.6f", oldScore)

	// 2. 热更新 boost → 10.0
	if err := client.UpdateMapping(index, map[string]interface{}{
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "text", "boost": 10.0, "analyzer": "jieba_std"},
		},
	}, ""); err != nil {
		t.Fatalf("update mapping: %v", err)
	}
	time.Sleep(500 * time.Millisecond)

	// 3. 用新 boost 写新文档
	client.Bulk(index, []map[string]interface{}{{"_id": "new", "name": "换发动机后胶垫"}}, "")
	time.Sleep(1500 * time.Millisecond)
	newScore := searchScore(t, client, index, "发动机")
	t.Logf("new doc (boost=10.0): %.6f", newScore)

	if newScore > oldScore*1.5 {
		t.Logf("BOOST HOT RELOAD WORKS: new doc %.6f >> old doc %.6f", newScore, oldScore)
	} else {
		t.Logf("BOOST HOT RELOAD DOES NOT WORK: old=%.6f new=%.6f (need >1.5x)", oldScore, newScore)
	}
}

// TestRealZinc_QueryLevelBoost 验证查询期 boost（替代方案）
func TestRealZinc_QueryLevelBoost(t *testing.T) {
	if !pingRealZinc(t) {
		t.Skip("real zinc not available")
	}
	client := NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {"http://localhost:4081"}},
		Default:  "default",
		Username: "admin",
		Password: "Complexpass#123",
	})

	index := fmt.Sprintf("sm_qboost_%d", time.Now().UnixNano())
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
		{"_id": "1", "name": "发动机维修", "content": "发动机是汽车核心部件"},
		{"_id": "2", "name": "轮胎更换", "content": "发动机皮带需要定期检查"},
	}, "")
	time.Sleep(1500 * time.Millisecond)

	// 查询期 boost：name^5 vs content^1
	resp, err := client.Search(index, map[string]interface{}{
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

	hits := resp["hits"].(map[string]interface{})["hits"].([]interface{})
	for _, h := range hits {
		hit := h.(map[string]interface{})
		source := hit["_source"].(map[string]interface{})
		name := source["name"].(string)
		t.Logf("doc %s (%s): score=%.6f", hit["_id"], name, hit["_score"].(float64))
	}

	if len(hits) == 2 {
		doc1Score := hits[0].(map[string]interface{})["_score"].(float64)
		doc2Score := hits[1].(map[string]interface{})["_score"].(float64)
		// doc1 在 name 字段匹配（boost=5），doc2 在 content 字段匹配（boost=1）
		if doc1Score > doc2Score {
			t.Logf("QUERY-LEVEL BOOST WORKS: name^5 (%.6f) > content (%.6f)", doc1Score, doc2Score)
		}
	}
}
