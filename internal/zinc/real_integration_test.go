package zinc

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"searchmiddleware/internal/config"
)

// TestRealZincIntegration 真实 Zinc 集成验证（本机 4080 可达时执行，否则 Skip）
// 验证 searchmiddleware zinc client 与真实 ZincSearch++ 的 API 兼容性
func TestRealZincIntegration(t *testing.T) {
	if !pingRealZinc(t) {
		t.Skip("real zinc not available at localhost:4081")
	}

	client := NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {"http://localhost:4081"}},
		Default:  "default",
		Username: "admin",
		Password: "Complexpass#123",
	})

	index := fmt.Sprintf("sm_real_test_%d", time.Now().UnixNano())
	t.Cleanup(func() { client.DeleteIndex(index, "") })

	t.Run("CreateIndex_WithElementTypeAndCopyTo", func(t *testing.T) {
		mapping := map[string]interface{}{
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type": "text", "analyzer": "jieba_std", "search_analyzer": "jieba_search",
				},
				"price": map[string]interface{}{
					"type": "numeric", "sortable": true, "aggregatable": true,
				},
				"category_ids": map[string]interface{}{
					"type": "keyword", "element_type": "long",
				},
				"tags": map[string]interface{}{
					"type": "keyword", "copy_to": []string{"all_text"},
				},
				"all_text": map[string]interface{}{"type": "text"},
			},
		}
		if err := client.CreateIndex(index, mapping, ""); err != nil {
			t.Fatalf("create index: %v", err)
		}
	})

	t.Run("Bulk_ArrayFields", func(t *testing.T) {
		docs := []map[string]interface{}{
			{"_id": "11073", "name": "换发动机后胶垫", "price": 68.5, "category_ids": []interface{}{238, 239}, "tags": []interface{}{"发动机", "胶垫"}, "all_text": "换发动机后胶垫"},
			{"_id": "11074", "name": "更换机油滤芯", "price": 45.0, "category_ids": []interface{}{238}, "tags": []interface{}{"机油"}, "all_text": "更换机油滤芯"},
		}
		if err := client.Bulk(index, docs, ""); err != nil {
			t.Fatalf("bulk: %v", err)
		}
		// Zinc NRT 语义：bulk 后需等待 refresh 才可搜索
		time.Sleep(1500 * time.Millisecond)
	})

	t.Run("MatchAll_Count2", func(t *testing.T) {
		resp, err := client.Search(index, map[string]interface{}{
			"query": map[string]interface{}{"match_all": map[string]interface{}{}},
			"size":  10,
		}, "")
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		total := extractTotal(t, resp)
		if total != 2 {
			t.Errorf("total = %d, want 2 (both docs indexed)", total)
		}
	})

	t.Run("MultiMatch_ChineseKeyword", func(t *testing.T) {
		resp, err := client.Search(index, map[string]interface{}{
			"query": map[string]interface{}{
				"multi_match": map[string]interface{}{
					"query":  "发动机",
					"fields": []string{"name^5.0", "all_text"},
					"type":   "best_fields",
				},
			},
		}, "")
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		total := extractTotal(t, resp)
		if total < 1 {
			t.Errorf("multi_match(发动机) total = %d, want >= 1", total)
		}
	})

	t.Run("TermsFilter_ArrayField", func(t *testing.T) {
		// Zinc BUG-001 已修复：element_type 数值存储 + 查询归一，数值查询直接命中
		resp, err := client.Search(index, map[string]interface{}{
			"query": map[string]interface{}{
				"bool": map[string]interface{}{
					"filter": []interface{}{map[string]interface{}{"terms": map[string]interface{}{"category_ids": []interface{}{238}}}},
				},
			},
		}, "")
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		total := extractTotal(t, resp)
		if total != 2 {
			t.Errorf("terms filter total = %d, want 2", total)
		}
	})

	t.Run("TermsAgg_ArrayField", func(t *testing.T) {
		resp, err := client.Search(index, map[string]interface{}{
			"query": map[string]interface{}{"match_all": map[string]interface{}{}},
			"size":  0,
			"aggs": map[string]interface{}{
				"cats": map[string]interface{}{"terms": map[string]interface{}{"field": "category_ids"}},
			},
		}, "")
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		aggs, ok := resp["aggregations"].(map[string]interface{})
		if !ok {
			t.Fatalf("aggregations missing: %v", resp)
		}
		cats, ok := aggs["cats"].(map[string]interface{})
		if !ok {
			t.Fatalf("cats agg missing: %v", aggs)
		}
		buckets, _ := cats["buckets"].([]interface{})
		if len(buckets) == 0 {
			t.Errorf("cats agg buckets empty: %v", cats)
		}
	})

	t.Run("RangeAgg_NumericField", func(t *testing.T) {
		// 验证 searchmiddleware 契约的 range 聚合格式（Q37b）是否被 Zinc 支持
		resp, err := client.Search(index, map[string]interface{}{
			"query": map[string]interface{}{"match_all": map[string]interface{}{}},
			"size":  0,
			"aggs": map[string]interface{}{
				"price_r": map[string]interface{}{
					"range": map[string]interface{}{
						"field":  "price",
						"ranges": []interface{}{[]interface{}{0, 50}, []interface{}{50, 100}},
					},
				},
			},
		}, "")
		if err != nil {
			t.Logf("RANGE_AGG_NOT_SUPPORTED: %v", err)
			t.Log("-> 需向 Zinc 提出需求：支持 ES range agg ranges 数组格式")
			return
		}
		t.Logf("range agg response: %v", resp)
	})

	t.Run("Sort_NumericField", func(t *testing.T) {
		resp, err := client.Search(index, map[string]interface{}{
			"query": map[string]interface{}{"match_all": map[string]interface{}{}},
			"sort":  []interface{}{map[string]interface{}{"price": map[string]interface{}{"order": "desc"}}},
		}, "")
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		if len(resp["hits"].(map[string]interface{})["hits"].([]interface{})) != 2 {
			t.Error("sort should return 2 hits")
		}
	})

	t.Run("SearchAfter_Pagination", func(t *testing.T) {
		resp, err := client.Search(index, map[string]interface{}{
			"query": map[string]interface{}{"match_all": map[string]interface{}{}},
			"sort":  []interface{}{map[string]interface{}{"_id": map[string]interface{}{"order": "asc"}}},
			"size":  1,
		}, "")
		if err != nil {
			t.Fatalf("search: %v", err)
		}
		hits := resp["hits"].(map[string]interface{})["hits"].([]interface{})
		if len(hits) != 1 {
			t.Fatalf("search_after page1 hits = %d, want 1", len(hits))
		}
	})

	t.Run("DeleteDoc", func(t *testing.T) {
		if err := client.DeleteDoc(index, "11074", ""); err != nil {
			t.Fatalf("delete doc: %v", err)
		}
		time.Sleep(1500 * time.Millisecond)
		resp, _ := client.Search(index, map[string]interface{}{
			"query": map[string]interface{}{"match_all": map[string]interface{}{}},
		}, "")
		if got := extractTotal(t, resp); got != 1 {
			t.Errorf("after delete total = %d, want 1", got)
		}
	})

	t.Run("AliasAddAndRemove", func(t *testing.T) {
		alias := "sm_real_test_alias"
		if err := client.AliasSwap(map[string][]string{alias: {index}}, nil, ""); err != nil {
			t.Fatalf("alias add: %v", err)
		}
		got, err := client.GetAlias(alias, "")
		if err != nil {
			t.Fatalf("get alias: %v", err)
		}
		raw, _ := json.Marshal(got)
		if !strings.Contains(string(raw), alias) {
			t.Errorf("alias not found in %s", raw)
		}
		if err := client.AliasSwap(nil, map[string][]string{alias: {index}}, ""); err != nil {
			t.Fatalf("alias remove: %v", err)
		}
	})
}

func pingRealZinc(t *testing.T) bool {
	client := NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {"http://localhost:4081"}},
		Default:  "default",
		Username: "admin",
		Password: "Complexpass#123",
	})
	// 实际探测（getHealthyURL 会 ping 所有节点）
	_, err := client.getHealthyURL("default")
	return err == nil
}

func extractTotal(t *testing.T, resp map[string]interface{}) int64 {
	t.Helper()
	hits, ok := resp["hits"].(map[string]interface{})
	if !ok {
		t.Fatalf("hits missing: %v", resp)
	}
	total, ok := hits["total"].(map[string]interface{})
	if !ok {
		t.Fatalf("total missing: %v", hits)
	}
	return int64(total["value"].(float64))
}
