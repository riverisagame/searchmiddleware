package zinc

import (
	"fmt"
	"testing"
	"time"

	"searchmiddleware/internal/config"
)

// TestRealZinc_KeywordArrayStringVsNumeric 验证 element_type 语义：
// keyword+element_type:long 数组字段，数值查询 vs 字符串查询
func TestRealZinc_KeywordArrayStringVsNumeric(t *testing.T) {
	if !pingRealZinc(t) {
		t.Skip("real zinc not available")
	}
	client := NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {"http://localhost:4080"}},
		Default:  "default",
		Username: "admin",
		Password: "Complexpass#123",
	})
	index := fmt.Sprintf("sm_kwarr_%d", time.Now().UnixNano())
	t.Cleanup(func() { client.DeleteIndex(index, "") })

	if err := client.CreateIndex(index, map[string]interface{}{
		"properties": map[string]interface{}{
			"category_ids": map[string]interface{}{"type": "keyword", "element_type": "long"},
		},
	}, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := client.Bulk(index, []map[string]interface{}{
		{"_id": "1", "category_ids": []interface{}{238, 239}},
	}, ""); err != nil {
		t.Fatalf("bulk: %v", err)
	}
	time.Sleep(1500 * time.Millisecond)

	// 数值查询
	resp, err := client.Search(index, map[string]interface{}{
		"query": map[string]interface{}{"terms": map[string]interface{}{"category_ids": []interface{}{238}}},
	}, "")
	if err != nil {
		t.Fatalf("numeric search: %v", err)
	}
	numericTotal := extractTotal(t, resp)
	t.Logf("numeric terms total = %d", numericTotal)

	// 字符串查询
	resp2, err := client.Search(index, map[string]interface{}{
		"query": map[string]interface{}{"terms": map[string]interface{}{"category_ids": []interface{}{"238"}}},
	}, "")
	if err != nil {
		t.Fatalf("string search: %v", err)
	}
	stringTotal := extractTotal(t, resp2)
	t.Logf("string terms total = %d", stringTotal)

	if numericTotal == 0 && stringTotal >= 1 {
		t.Log("CONFIRMED_BUG: keyword+element_type 数组字段存字符串 term，数值查询不命中")
		t.Log("-> 需向 Zinc 提出需求：element_type 应参与存储归一化（按 long 存）或查询归一化（数值自动转字符串）")
	}
}
