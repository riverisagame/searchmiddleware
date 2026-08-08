package zinc

import (
	"fmt"
	"testing"
	"time"
)

// TestRealZinc_SynonymBaseline 对比：搜原词 vs 搜同义词
func TestRealZinc_SynonymBaseline(t *testing.T) {
	client := newC4081()
	probe4081(t, client)

	index := fmt.Sprintf("sm_synb_%d", time.Now().UnixNano())
	t.Cleanup(func() { client.DeleteIndex(index, "") })
	client.CreateIndex(index, map[string]interface{}{
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "text", "analyzer": "jieba_std", "search_analyzer": "jieba_search"},
		},
	}, "")
	client.Bulk(index, []map[string]interface{}{
		{"_id": "1", "name": "手机维修服务"},
	}, "")
	waitDocs(t, client, index)
	client.ReloadSynonym("")

	search := func(keyword string) float64 {
		resp, err := client.Search(index, map[string]interface{}{
			"query": map[string]interface{}{"match": map[string]interface{}{"name": keyword}},
		}, "")
		if err != nil {
			t.Fatalf("search %q: %v", keyword, err)
		}
		return totalOf(t, resp)
	}

	t1 := search("手机")       // 原词
	t2 := search("移动电话")    // 同义词（jieba 切分为 移动/电话）
	t3 := search("handset")    // 同义词（英文，单 token）
	t.Logf("原词'手机': %v | 同义词'移动电话': %v | 同义词'handset': %v", t1, t2, t3)

	if t1 >= 1 {
		t.Log("BASELINE OK: 原词命中")
	}
	if t3 >= 1 {
		t.Log("SYNONYM_HANDSET OK: 英文同义词命中（扩展 token 参与查询）")
	} else {
		t.Log("SYNONYM_HANDSET MISS")
	}
	if t2 >= 1 {
		t.Log("SYNONYM_CN OK: 中文同义词命中")
	} else {
		t.Log("SYNONYM_CN MISS: 中文同义词（分词粒度）未命中")
	}
}
