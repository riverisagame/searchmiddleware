package zinc

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"searchmiddleware/internal/config"
)

// TestRealZinc_BoolFilterNested 验证 filter 数组内 bool 子查询（ES 标准支持）是否被 Zinc 接受
func TestRealZinc_BoolFilterNested(t *testing.T) {
	client := NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {"http://localhost:4081"}},
		Default:  "default",
		Username: "admin",
		Password: "Complexpass#123",
	})
	if _, err := client.getHealthyURL("default"); err != nil {
		t.Skip("real zinc not available")
	}

	index := fmt.Sprintf("sm_bfn_%d", time.Now().UnixNano())
	t.Cleanup(func() { client.DeleteIndex(index, "") })
	client.CreateIndex(index, map[string]interface{}{
		"properties": map[string]interface{}{
			"category_ids": map[string]interface{}{"type": "keyword", "element_type": "long"},
		},
	}, "")
	client.Bulk(index, []map[string]interface{}{
		{"_id": "1", "category_ids": []interface{}{238}},
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
			hits, _ := respAll["hits"].(map[string]interface{})
			total, _ := hits["total"].(map[string]interface{})
			docTotal = int64(total["value"].(float64))
			if docTotal > 0 {
				break
			}
		}
	}
	t.Logf("docs total=%d", docTotal)

	cases := map[string]map[string]interface{}{
		// A: 裸 terms（中间件当前用法，已知 OK）
		"A_bare_terms": {
			"query": map[string]interface{}{
				"bool": map[string]interface{}{
					"filter": []interface{}{map[string]interface{}{"terms": map[string]interface{}{"category_ids": []interface{}{"238"}}}},
				},
			},
		},
		// B: filter 数组内 bool 包装（ES 标准：filter 可含 bool）
		"B_nested_bool": {
			"query": map[string]interface{}{
				"bool": map[string]interface{}{
					"filter": []interface{}{
						map[string]interface{}{"bool": map[string]interface{}{"must": []interface{}{map[string]interface{}{"terms": map[string]interface{}{"category_ids": []interface{}{"238"}}}}}},
					},
				},
			},
		},
	}

	for name, body := range cases {
		resp, err := client.Search(index, body, "")
		if err != nil {
			t.Logf("%s: ERROR %v", name, err)
		} else {
			hits, _ := resp["hits"].(map[string]interface{})
			total, _ := hits["total"].(map[string]interface{})
			t.Logf("%s: OK total=%v", name, total["value"])
		}
	}

	_ = json.Marshal
	_ = time.Now
}
