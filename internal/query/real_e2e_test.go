package query

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"searchmiddleware/internal/config"
	"searchmiddleware/internal/zinc"
)

// TestRealZinc_QueryBuilderBoost 真实 Zinc 端到端：QueryBuilder 权重 boost 生效验证
func TestRealZinc_QueryBuilderBoost(t *testing.T) {
	zc := zinc.NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {"http://localhost:4080"}},
		Default:  "default",
		Username: "admin",
		Password: "Complexpass#123",
	})
	if !zincProbe(t, zc) {
		t.Skip("real zinc not available at localhost:4080")
	}

	index := fmt.Sprintf("sm_qb_e2e_%d", time.Now().UnixNano())
	t.Cleanup(func() { zc.DeleteIndex(index, "") })

	if err := zc.CreateIndex(index, map[string]interface{}{
		"properties": map[string]interface{}{
			"maintenance_name": map[string]interface{}{"type": "text", "analyzer": "jieba_std", "search_analyzer": "jieba_search"},
			"sub_title":        map[string]interface{}{"type": "text", "analyzer": "jieba_std", "search_analyzer": "jieba_search"},
			"category_names":   map[string]interface{}{"type": "text", "analyzer": "jieba_std", "search_analyzer": "jieba_search"},
		},
	}, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	// doc1: 关键词在 maintenance_name（boost 5）; doc2: 关键词仅在 category_names（boost 1）
	if err := zc.Bulk(index, []map[string]interface{}{
		{"_id": "1", "maintenance_name": "发动机维修", "sub_title": "专业服务", "category_names": "普通分类"},
		{"_id": "2", "maintenance_name": "轮胎更换", "sub_title": "常规项目", "category_names": "发动机相关配件"},
	}, ""); err != nil {
		t.Fatalf("bulk: %v", err)
	}
	time.Sleep(1500 * time.Millisecond)

	// 用 QueryBuilder 构建查询（维护配置：maintenance_name=5, sub_title=3, category_names=1）
	indexCfg := &config.IndexConfig{
		Index: config.IndexIndexConfig{
			Boost: map[string]float64{
				"maintenance_name": 5.0,
				"sub_title":        3.0,
				"category_names":   1.0,
			},
			Fields: map[string]config.FieldConfig{
				"maintenance_name": {Type: "text", Searchable: true},
				"sub_title":        {Type: "text", Searchable: true},
				"category_names":   {Type: "text", Searchable: true},
			},
		},
	}
	qb := NewQueryBuilder(indexCfg, nil)
	body, err := qb.Build(SearchRequest{Keyword: "发动机", Limit: 10})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	t.Logf("query body: %v", body)

	// 诊断：去掉 fuzziness 后重试
	bodyNoFuzzy := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []interface{}{
					map[string]interface{}{"match": map[string]interface{}{"maintenance_name": map[string]interface{}{"query": "发动机", "boost": 5.0}}},
					map[string]interface{}{"match": map[string]interface{}{"sub_title": map[string]interface{}{"query": "发动机", "boost": 3.0}}},
					map[string]interface{}{"match": map[string]interface{}{"category_names": map[string]interface{}{"query": "发动机", "boost": 1.0}}},
				},
			},
		},
		"size": 10,
	}
	respDiag, errDiag := zc.Search(index, bodyNoFuzzy, "")
	if errDiag != nil {
		t.Logf("diag search error: %v", errDiag)
	} else {
		diagHits, _ := respDiag["hits"].(map[string]interface{})["hits"].([]interface{})
		t.Logf("diag (no fuzzy) hits: %d", len(diagHits))
	}

	// 诊断2：QueryBuilder body 但去掉空 filter/should 数组
	bodyClean := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []interface{}{
					map[string]interface{}{
						"bool": map[string]interface{}{
							"minimum_should_match": 1,
							"should": []interface{}{
								map[string]interface{}{"match": map[string]interface{}{"maintenance_name": map[string]interface{}{"query": "发动机", "boost": 5.0}}},
								map[string]interface{}{"match": map[string]interface{}{"sub_title": map[string]interface{}{"query": "发动机", "boost": 3.0}}},
								map[string]interface{}{"match": map[string]interface{}{"category_names": map[string]interface{}{"query": "发动机", "boost": 1.0}}},
							},
						},
					},
				},
			},
		},
		"size": 10,
	}
	respClean, errClean := zc.Search(index, bodyClean, "")
	if errClean != nil {
		t.Logf("clean search error: %v", errClean)
	} else {
		cleanHits, _ := respClean["hits"].(map[string]interface{})["hits"].([]interface{})
		t.Logf("clean (must+bool.should) hits: %d", len(cleanHits))
	}

	// 诊断3：手工 bool should（2 字段）+ minimum_should_match
	bodyMSM := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"should": []interface{}{
					map[string]interface{}{"match": map[string]interface{}{"maintenance_name": map[string]interface{}{"query": "发动机", "boost": 5.0}}},
					map[string]interface{}{"match": map[string]interface{}{"category_names": map[string]interface{}{"query": "发动机", "boost": 1.0}}},
				},
				"minimum_should_match": 1,
			},
		},
		"size": 10,
	}
	respMSM, errMSM := zc.Search(index, bodyMSM, "")
	if errMSM != nil {
		t.Logf("msm search error: %v", errMSM)
	} else {
		msmHits, _ := respMSM["hits"].(map[string]interface{})["hits"].([]interface{})
		t.Logf("msm (should+minimum_should_match) hits: %d", len(msmHits))
		for _, h := range msmHits {
			hit := h.(map[string]interface{})
			src := hit["_source"].(map[string]interface{})
			t.Logf("  msm doc %s (%s): %.4f", hit["_id"], src["maintenance_name"], hit["_score"].(float64))
		}
	}

	resp, err := zc.Search(index, body, "")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	hitsRaw, _ := resp["hits"].(map[string]interface{})
	if hitsRaw == nil {
		t.Fatalf("no hits in response: %v", resp)
	}
	hits, _ := hitsRaw["hits"].([]interface{})
	t.Logf("raw hits count: %d", len(hits))
	if len(hits) != 2 {
		t.Fatalf("hits = %d, want 2", len(hits))
	}
	doc1Score := hits[0].(map[string]interface{})["_score"].(float64)
	doc2Score := hits[1].(map[string]interface{})["_score"].(float64)
	t.Logf("doc1 (name^5): %.4f, doc2 (category^1): %.4f, ratio %.1fx", doc1Score, doc2Score, doc1Score/doc2Score)

	if doc1Score <= doc2Score {
		t.Errorf("BUG-003 规避失败：name boost=5 的 doc1 (%.4f) 应高于 category boost=1 的 doc2 (%.4f)", doc1Score, doc2Score)
	} else {
		t.Logf("QueryBuilder 权重生效：boost=5 字段命中文档排序优先 (%.1fx)", doc1Score/doc2Score)
	}
}

// zincProbe 实际探测 Zinc 可达性：可达时查询不存在索引返回 404（err 非网络类）
func zincProbe(t *testing.T, c *zinc.Client) bool {
	t.Helper()
	_, err := c.Search("sm_probe_nonexistent", map[string]interface{}{
		"query": map[string]interface{}{"match_all": map[string]interface{}{}},
	}, "")
	if err == nil {
		return false
	}
	msg := fmt.Sprintf("%v", err)
	return !strings.Contains(msg, "no healthy nodes") &&
		!strings.Contains(msg, "dial tcp") &&
		!strings.Contains(msg, "refused") &&
		!strings.Contains(msg, "timeout")
}
