package zinc

import (
	"fmt"
	"testing"
	"time"

	"searchmiddleware/internal/config"
)

// TestRealZinc_BUG004_FuzzinessCJK 验证 BUG-004 修复：中文 + fuzziness 不再 0 命中
func TestRealZinc_BUG004_FuzzinessCJK(t *testing.T) {
	if !pingRealZinc(t) {
		t.Skip("real zinc not available")
	}
	client := NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {"http://localhost:4080"}},
		Default:  "default",
		Username: "admin",
		Password: "Complexpass#123",
	})

	index := fmt.Sprintf("sm_fuzzy_%d", time.Now().UnixNano())
	t.Cleanup(func() { client.DeleteIndex(index, "") })

	if err := client.CreateIndex(index, map[string]interface{}{
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "text", "analyzer": "jieba_std", "search_analyzer": "jieba_search"},
		},
	}, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	client.Bulk(index, []map[string]interface{}{
		{"_id": "1", "name": "换发动机后胶垫"},
		{"_id": "2", "name": "更换机油滤芯"},
	}, "")
	time.Sleep(1500 * time.Millisecond)

	// 1. 不带 fuzziness（基线，应命中）
	base, err := client.Search(index, map[string]interface{}{
		"query": map[string]interface{}{"match": map[string]interface{}{"name": "发动机"}},
	}, "")
	if err != nil {
		t.Fatalf("base search: %v", err)
	}
	baseTotal := extractTotal(t, base)
	t.Logf("基线（无 fuzziness）: %d 命中", baseTotal)

	// 2. 带 fuzziness AUTO（修复前 0 命中，修复后应命中）
	fuzzy, err := client.Search(index, map[string]interface{}{
		"query": map[string]interface{}{"match": map[string]interface{}{"name": map[string]interface{}{"query": "发动机", "fuzziness": "AUTO"}}},
	}, "")
	if err != nil {
		t.Fatalf("fuzzy search: %v", err)
	}
	fuzzyTotal := extractTotal(t, fuzzy)
	t.Logf("fuzziness AUTO: %d 命中", fuzzyTotal)

	if fuzzyTotal >= 1 {
		t.Logf("BUG-004 FIXED: 中文 + fuzziness 不再 0 命中（%d 条）", fuzzyTotal)
	} else {
		t.Errorf("BUG-004 NOT FIXED: 中文 + fuzziness AUTO 仍 0 命中")
	}
}

// TestRealZinc_BUG002_MappingBoostQueryTime 验证 BUG-002 方案 B：mapping 配置 boost 查询期生效
func TestRealZinc_BUG002_MappingBoostQueryTime(t *testing.T) {
	if !pingRealZinc(t) {
		t.Skip("real zinc not available")
	}
	client := NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {"http://localhost:4080"}},
		Default:  "default",
		Username: "admin",
		Password: "Complexpass#123",
	})

	ts := time.Now().UnixNano()
	indexA := fmt.Sprintf("sm_mb_a_%d", ts) // name boost=1
	indexB := fmt.Sprintf("sm_mb_b_%d", ts) // name boost=10
	t.Cleanup(func() {
		client.DeleteIndex(indexA, "")
		client.DeleteIndex(indexB, "")
	})

	doc := map[string]interface{}{"_id": "1", "name": "换发动机后胶垫"}

	client.CreateIndex(indexA, map[string]interface{}{
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "text", "boost": 1.0, "analyzer": "jieba_std"},
		},
	}, "")
	client.Bulk(indexA, []map[string]interface{}{doc}, "")
	time.Sleep(1500 * time.Millisecond)

	client.CreateIndex(indexB, map[string]interface{}{
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "text", "boost": 10.0, "analyzer": "jieba_std"},
		},
	}, "")
	client.Bulk(indexB, []map[string]interface{}{doc}, "")
	time.Sleep(1500 * time.Millisecond)

	// 用 match 查询（match.go:321-334 会应用 prop.Boost）
	searchWith := func(idx string) float64 {
		resp, err := client.Search(idx, map[string]interface{}{
			"query": map[string]interface{}{"match": map[string]interface{}{"name": "发动机"}},
		}, "")
		if err != nil {
			t.Fatalf("search %s: %v", idx, err)
		}
		hits, _ := resp["hits"].(map[string]interface{})["hits"].([]interface{})
		if len(hits) == 0 {
			t.Fatalf("no hits for %s", idx)
		}
		return hits[0].(map[string]interface{})["_score"].(float64)
	}

	scoreA := searchWith(indexA)
	scoreB := searchWith(indexB)
	t.Logf("mapping boost=1.0: %.4f | mapping boost=10.0: %.4f | ratio %.1fx", scoreA, scoreB, scoreB/scoreA)

	if scoreB > scoreA*1.5 {
		t.Logf("BUG-002 方案B FIXED: mapping boost 查询期生效 (%.1fx)", scoreB/scoreA)
	} else {
		t.Errorf("BUG-002 方案B NOT FIXED: mapping boost 未在查询期生效 (ratio %.1f)", scoreB/scoreA)
	}
}

// TestRealZinc_BUG003_MultiMatchCaretBoost 验证 BUG-003 修复：multi_match name^5 命中不丢 + 权重生效
func TestRealZinc_BUG003_MultiMatchCaretBoost(t *testing.T) {
	if !pingRealZinc(t) {
		t.Skip("real zinc not available")
	}
	client := NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {"http://localhost:4080"}},
		Default:  "default",
		Username: "admin",
		Password: "Complexpass#123",
	})

	index := fmt.Sprintf("sm_mm_%d", time.Now().UnixNano())
	t.Cleanup(func() { client.DeleteIndex(index, "") })

	if err := client.CreateIndex(index, map[string]interface{}{
		"properties": map[string]interface{}{
			"name":    map[string]interface{}{"type": "text", "analyzer": "jieba_std", "search_analyzer": "jieba_search"},
			"content": map[string]interface{}{"type": "text", "analyzer": "jieba_std", "search_analyzer": "jieba_search"},
		},
	}, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	// doc1 只在 name 命中；doc2 只在 content 命中
	client.Bulk(index, []map[string]interface{}{
		{"_id": "1", "name": "发动机维修", "content": "汽车保养服务项目介绍"},
		{"_id": "2", "name": "轮胎更换", "content": "发动机皮带定期检查知识"},
	}, "")
	time.Sleep(1500 * time.Millisecond)

	// 1. 无 boost 基线：两个字段都命中
	baseResp, err := client.Search(index, map[string]interface{}{
		"query": map[string]interface{}{
			"multi_match": map[string]interface{}{"query": "发动机", "fields": []string{"name", "content"}, "type": "best_fields"},
		},
	}, "")
	if err != nil {
		t.Fatalf("base search: %v", err)
	}
	baseHits, _ := baseResp["hits"].(map[string]interface{})["hits"].([]interface{})
	t.Logf("基线 multi_match: %d 命中", len(baseHits))

	// 2. name^5：doc1 应命中（修复前丢失），且分数应高于 doc2
	boostResp, err := client.Search(index, map[string]interface{}{
		"query": map[string]interface{}{
			"multi_match": map[string]interface{}{"query": "发动机", "fields": []string{"name^5", "content"}, "type": "best_fields"},
		},
	}, "")
	if err != nil {
		t.Fatalf("boost search: %v", err)
	}
	boostHits, _ := boostResp["hits"].(map[string]interface{})["hits"].([]interface{})
	t.Logf("name^5 multi_match: %d 命中", len(boostHits))

	var nameScore, contentScore float64
	doc1Found, doc2Found := false, false
	for _, h := range boostHits {
		hit := h.(map[string]interface{})
		src := hit["_source"].(map[string]interface{})
		sc := hit["_score"].(float64)
		t.Logf("  doc %s (%s): %.4f", hit["_id"], src["name"], sc)
		if hit["_id"] == "1" {
			nameScore, doc1Found = sc, true
		} else {
			contentScore, doc2Found = sc, true
		}
	}

	if !doc1Found {
		t.Errorf("BUG-003 NOT FIXED: doc1（name 字段命中）在 name^5 查询中丢失")
		return
	}
	if !doc2Found {
		t.Errorf("BUG-003 部分问题: doc2（content 字段命中）未返回")
		return
	}
	t.Logf("name^5=%.4f vs content=%.4f, ratio %.1fx", nameScore, contentScore, nameScore/contentScore)
	if nameScore > contentScore*1.5 {
		t.Logf("BUG-003 FIXED: name^5 命中不丢且权重生效 (%.1fx)", nameScore/contentScore)
	} else {
		t.Errorf("BUG-003 部分修复: doc1 命中但权重未生效 (ratio %.1f)", nameScore/contentScore)
	}
}
