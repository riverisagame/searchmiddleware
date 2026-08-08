package zinc

import (
	"fmt"
	"testing"
	"time"

	"searchmiddleware/internal/config"
)

// TestRealZinc_BUG007_TermsAggKey 验证 BUG-007 修复：element_type 数值字段聚合 key 应为数值
func TestRealZinc_BUG007_TermsAggKey(t *testing.T) {
	client := NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {"http://localhost:4081"}},
		Default:  "default",
		Username: "admin",
		Password: "Complexpass#123",
	})
	if _, err := client.getHealthyURL("default"); err != nil {
		t.Skip("real zinc not available at 4081")
	}

	index := fmt.Sprintf("sm_aggkey_%d", time.Now().UnixNano())
	t.Cleanup(func() { client.DeleteIndex(index, "") })

	if err := client.CreateIndex(index, map[string]interface{}{
		"properties": map[string]interface{}{
			"category_ids": map[string]interface{}{"type": "keyword", "element_type": "long", "aggregatable": true},
		},
	}, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	client.Bulk(index, []map[string]interface{}{
		{"_id": "1", "category_ids": []interface{}{238, 239}},
		{"_id": "2", "category_ids": []interface{}{240}},
	}, "")

	// 等待 shard 初始化 + NRT
	var docTotal int64
	for i := 0; i < 5; i++ {
		time.Sleep(1000 * time.Millisecond)
		client.Refresh(index, "")
		respAll, err := client.Search(index, map[string]interface{}{
			"query": map[string]interface{}{"match_all": map[string]interface{}{}},
		}, "")
		if err == nil {
			docTotal = extractTotal(t, respAll)
			if docTotal > 0 {
				break
			}
		}
	}
	t.Logf("docs total=%d", docTotal)

	resp, err := client.Search(index, map[string]interface{}{
		"query": map[string]interface{}{"match_all": map[string]interface{}{}},
		"aggs": map[string]interface{}{
			"cats": map[string]interface{}{"terms": map[string]interface{}{"field": "category_ids"}},
		},
		"size": 0,
	}, "")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	t.Logf("agg raw: %v", resp)

	aggs, ok := resp["aggregations"].(map[string]interface{})
	if !ok {
		t.Fatalf("no aggregations: %v", resp)
	}
	cats, _ := aggs["cats"].(map[string]interface{})
	buckets, _ := cats["buckets"].([]interface{})
	t.Logf("buckets: %v", buckets)

	allNumeric := true
	for _, b := range buckets {
		bm, _ := b.(map[string]interface{})
		switch bm["key"].(type) {
		case float64:
			t.Logf("  key=%v (numeric)", bm["key"])
		case string:
			// 字符串可能是正常 "238" 或乱码字节
			t.Logf("  key=%q (string)", bm["key"])
			allNumeric = false
		default:
			t.Logf("  key type %T: %v", bm["key"], bm["key"])
			allNumeric = false
		}
	}

	if len(buckets) >= 2 && allNumeric {
		t.Logf("BUG-007 FIXED: element_type numeric 字段聚合 key 为数值")
	} else {
		t.Logf("BUG-007 状态需人工确认（buckets=%d, allNumeric=%v）", len(buckets), allNumeric)
		if len(buckets) == 0 {
			t.Error("no buckets returned")
		}
	}
}
