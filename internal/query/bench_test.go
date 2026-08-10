package query

import (
	"testing"

	"searchmiddleware/internal/config"
)

// 搜索热路径：QueryBuilder.Build（含同义词扩展 + filter + aggs + 高亮）
func BenchmarkQueryBuilderBuild(b *testing.B) {
	indexCfg := &config.IndexConfig{
		Index: config.IndexIndexConfig{
			Fields: map[string]config.FieldConfig{
				"name": {Type: "text", Analyzer: "jieba_std", SearchAnalyzer: "jieba_search"},
				"site_id": {Type: "numeric"},
				"price": {Type: "numeric", Sortable: true},
				"category_ids": {Type: "numeric", Agg: true},
			},
		},
	}
	synonyms := map[string][]string{
		"手机": {"移动电话", "handset"},
		"维修": {"保养", "修理"},
	}
	qb := NewQueryBuilder(indexCfg, synonyms)
	req := SearchRequest{
		Index:   "maintenance",
		Keyword: "手机维修",
		Filter: map[string]interface{}{
			"site_id":      float64(1),
			"category_ids": []interface{}{float64(238)},
			"price":        map[string]interface{}{"gte": float64(10), "lte": float64(100)},
		},
		Page:      1,
		Limit:     20,
		Sort:      "price:desc",
		Highlight: true,
		Aggs: map[string]interface{}{
			"categories": map[string]interface{}{"field": "category_ids", "size": 20},
			"price_ranges": map[string]interface{}{"field": "price", "ranges": [][]float64{{0, 100}, {100, 300}}},
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := qb.Build(req); err != nil {
			b.Fatal(err)
		}
	}
}

// 同义词扩展热路径
func BenchmarkBuildKeywordQuery(b *testing.B) {
	indexCfg := &config.IndexConfig{
		Index: config.IndexIndexConfig{Fields: map[string]config.FieldConfig{
			"name": {Type: "text", SearchAnalyzer: "jieba_search", Searchable: true},
		}},
	}
	qb := NewQueryBuilder(indexCfg, map[string][]string{
		"手机": {"移动电话", "handset", "mobile phone"},
		"维修": {"保养", "修理", "维护"},
	})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if got := qb.buildKeywordQuery("手机维修 发动机"); got == nil {
			b.Fatal("nil should")
		}
	}
}
