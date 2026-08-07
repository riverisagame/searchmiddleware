package query

import (
	"encoding/json"
	"testing"

	"searchmiddleware/internal/config"
)

func newTestIndexCfg() *config.IndexConfig {
	return &config.IndexConfig{
		Index: config.IndexIndexConfig{
			Boost: map[string]float64{
				"maintenance_name": 5.0,
				"sub_title":        3.0,
			},
			Fields: map[string]config.FieldConfig{
				"maintenance_name": {Type: "text", Searchable: true},
				"sub_title":        {Type: "text", Searchable: true},
				"category_names":   {Type: "text", Searchable: true},
			},
		},
	}
}

// TestBuildKeywordQuery_BoolShouldBoost 验证 BUG-003 规避后的查询结构：
// 每个字段独立 match + 查询级 boost，不再使用 multi_match field^boost
func TestBuildKeywordQuery_BoolShouldBoost(t *testing.T) {
	qb := NewQueryBuilder(newTestIndexCfg(), nil)
	body, err := qb.Build(SearchRequest{Keyword: "发动机", Limit: 10})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	raw, _ := json.Marshal(body)
	jsonStr := string(raw)

	// 1. 不应包含 multi_match（BUG-003 触发源）
	if contains(jsonStr, "multi_match") {
		t.Errorf("query should not use multi_match: %s", jsonStr)
	}
	// 2. 不应包含 field^boost 语法（Zinc 未解析）
	if contains(jsonStr, "maintenance_name^") {
		t.Errorf("query should not use field^boost syntax: %s", jsonStr)
	}
	// 3. 应为 bool.should 多 match，每字段带查询级 boost
	if !contains(jsonStr, `"boost":5`) {
		t.Errorf("maintenance_name should have boost 5: %s", jsonStr)
	}
	if !contains(jsonStr, `"boost":3`) {
		t.Errorf("sub_title should have boost 3: %s", jsonStr)
	}
	if !contains(jsonStr, `"boost":1`) {
		t.Errorf("category_names should have boost 1: %s", jsonStr)
	}
	if !contains(jsonStr, "minimum_should_match") {
		t.Errorf("should have minimum_should_match: %s", jsonStr)
	}
}

// TestBuildKeywordQuery_NoBoost 无 boost 配置时所有字段 boost=1
func TestBuildKeywordQuery_NoBoost(t *testing.T) {
	cfg := newTestIndexCfg()
	cfg.Index.Boost = nil
	qb := NewQueryBuilder(cfg, nil)
	body, err := qb.Build(SearchRequest{Keyword: "机油", Limit: 10})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	raw, _ := json.Marshal(body)
	if !contains(string(raw), `"boost":1`) {
		t.Errorf("default boost should be 1: %s", raw)
	}
}

// TestBuildKeywordQuery_Synonym 同义词扩展为 bool should 结构
func TestBuildKeywordQuery_Synonym(t *testing.T) {
	qb := NewQueryBuilder(newTestIndexCfg(), map[string][]string{
		"手机": {"移动电话", "handset"},
	})
	body, err := qb.Build(SearchRequest{Keyword: "手机", Limit: 10})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	raw, _ := json.Marshal(body)
	if !contains(string(raw), "移动电话") {
		t.Errorf("synonym should be expanded: %s", raw)
	}
	if !contains(string(raw), `"boost":0.5`) {
		t.Errorf("synonym should have boost 0.5: %s", raw)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
