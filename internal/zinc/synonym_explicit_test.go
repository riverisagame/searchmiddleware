package zinc

import (
	"fmt"
	"testing"
	"time"
)

// TestRealZinc_SynonymExplicitAnalyzer 显式指定 analyzer 排查
func TestRealZinc_SynonymExplicitAnalyzer(t *testing.T) {
	client := newC4081()
	probe4081(t, client)

	index := fmt.Sprintf("sm_syna_%d", time.Now().UnixNano())
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

	search := func(keyword string, analyzer string) float64 {
		match := map[string]interface{}{"name": map[string]interface{}{"query": keyword}}
		if analyzer != "" {
			match["name"].(map[string]interface{})["analyzer"] = analyzer
		}
		resp, err := client.Search(index, map[string]interface{}{
			"query": map[string]interface{}{"match": match},
		}, "")
		if err != nil {
			t.Fatalf("search %q: %v", keyword, err)
		}
		return totalOf(t, resp)
	}

	t.Logf("handset(默认analyzer): %v", search("handset", ""))
	t.Logf("handset(显式jieba_search): %v", search("handset", "jieba_search"))
	t.Logf("handset(显式jieba_std): %v", search("handset", "jieba_std"))
	t.Logf("手机(默认): %v", search("手机", ""))
}
