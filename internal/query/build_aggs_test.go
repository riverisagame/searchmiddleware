package query

import (
	"encoding/json"
	"testing"

	"searchmiddleware/internal/config"
)

// TestBuildWithAggs 验证简写 aggs 被自动包装为 Zinc terms/range 结构
func TestBuildWithAggs(t *testing.T) {
	cfg := &config.IndexConfig{
		Index: config.IndexIndexConfig{
			Fields: map[string]config.FieldConfig{
				"name":         {Type: "text", Searchable: true},
				"category_ids": {Type: "keyword", Agg: true},
			},
		},
	}
	qb := NewQueryBuilder(cfg, nil)

	var aggs map[string]interface{}
	json.Unmarshal([]byte(`{"cats":{"field":"category_ids","size":20},"prices":{"field":"price","ranges":[[0,100],[100,300]]}}`), &aggs)

	body, err := qb.Build(SearchRequest{Keyword: "发动机", Aggs: aggs, Limit: 10})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	raw, _ := json.Marshal(body)
	jsonStr := string(raw)

	// terms 包装
	if !contains(jsonStr, `"terms":{"field":"category_ids"`) {
		t.Errorf("cats 应包装为 terms: %s", jsonStr)
	}
	// range 包装
	if !contains(jsonStr, `"range":{"field":"price"`) {
		t.Errorf("prices 应包装为 range: %s", jsonStr)
	}
	// took 透传（ParseResponse 需含 took）
	var resp map[string]interface{}
	json.Unmarshal([]byte(`{"hits":{"total":{"value":1},"hits":[]},"took":3,"aggregations":{"cats":{"buckets":[{"key":"238","doc_count":1}]}}}`), &resp)
	parsed, err := qb.ParseResponse(resp)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed["took"] == nil {
		t.Error("took 应透传到响应")
	}
	if parsed["aggs"] == nil {
		t.Error("aggs 应透传到响应")
	}
}
