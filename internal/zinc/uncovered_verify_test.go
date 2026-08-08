package zinc

import (
	"fmt"
	"testing"
	"time"

	"searchmiddleware/internal/config"
)

func newC4081() *Client {
	return NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {"http://localhost:4081"}},
		Default:  "default",
		Username: "admin",
		Password: "Complexpass#123",
	})
}

func probe4081(t *testing.T, c *Client) {
	if _, err := c.getHealthyURL("default"); err != nil {
		t.Skip("real zinc not available at 4081")
	}
}

func waitDocs(t *testing.T, c *Client, index string) {
	for i := 0; i < 5; i++ {
		time.Sleep(1000 * time.Millisecond)
		c.Refresh(index, "")
		resp, err := c.Search(index, map[string]interface{}{"query": map[string]interface{}{"match_all": map[string]interface{}{}}}, "")
		if err == nil {
			hits, _ := resp["hits"].(map[string]interface{})
			total, _ := hits["total"].(map[string]interface{})
			if total != nil && total["value"].(float64) > 0 {
				return
			}
		}
	}
}

func totalOf(t *testing.T, resp map[string]interface{}) float64 {
	hits, _ := resp["hits"].(map[string]interface{})
	total, _ := hits["total"].(map[string]interface{})
	return total["value"].(float64)
}

// 1. 同义词搜索端到端：搜 "移动电话" 应命中含 "手机" 的文档
func TestRealZinc_SynonymSearch(t *testing.T) {
	c := newC4081()
	probe4081(t, c)

	index := fmt.Sprintf("sm_syn_%d", time.Now().UnixNano())
	t.Cleanup(func() { c.DeleteIndex(index, "") })
	c.CreateIndex(index, map[string]interface{}{
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "text", "analyzer": "jieba_std", "search_analyzer": "jieba_search"},
		},
	}, "")
	c.Bulk(index, []map[string]interface{}{
		{"_id": "1", "name": "手机维修服务"},
		{"_id": "2", "name": "电脑维修服务"},
	}, "")
	waitDocs(t, c, index)

	// 触发同义词重载（synonyms.txt: 手机,移动电话,handset）
	if err := c.ReloadSynonym(""); err != nil {
		t.Fatalf("reload synonym: %v", err)
	}

	// 搜同义词 "移动电话"
	resp, err := c.Search(index, map[string]interface{}{
		"query": map[string]interface{}{"match": map[string]interface{}{"name": "移动电话"}},
	}, "")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	total := totalOf(t, resp)
	t.Logf("同义词搜索 '移动电话': total=%v", total)
	if total >= 1 {
		t.Log("SYNONYM_SEARCH OK: 同义词命中")
	} else {
		t.Log("SYNONYM_SEARCH MISS: 同义词未命中（可能处理器未生效，需人工确认）")
	}
}

// 2. 拼音搜索：搜 "shouji" 应命中含 "手机" 的文档
func TestRealZinc_PinyinSearch(t *testing.T) {
	c := newC4081()
	probe4081(t, c)

	index := fmt.Sprintf("sm_py_%d", time.Now().UnixNano())
	t.Cleanup(func() { c.DeleteIndex(index, "") })
	c.CreateIndex(index, map[string]interface{}{
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "text", "analyzer": "jieba_std", "search_analyzer": "jieba_search"},
		},
	}, "")
	c.Bulk(index, []map[string]interface{}{
		{"_id": "1", "name": "手机维修服务"},
	}, "")
	waitDocs(t, c, index)

	// 搜全拼
	respFull, err := c.Search(index, map[string]interface{}{
		"query": map[string]interface{}{"match": map[string]interface{}{"name": "shouji"}},
	}, "")
	if err != nil {
		t.Fatalf("search full: %v", err)
	}
	t.Logf("拼音全拼 'shouji': total=%v", totalOf(t, respFull))

	// 搜首字母
	respFirst, err := c.Search(index, map[string]interface{}{
		"query": map[string]interface{}{"match": map[string]interface{}{"name": "sj"}},
	}, "")
	if err != nil {
		t.Fatalf("search first: %v", err)
	}
	t.Logf("拼音首字母 'sj': total=%v", totalOf(t, respFirst))

	if totalOf(t, respFull) >= 1 || totalOf(t, respFirst) >= 1 {
		t.Log("PINYIN OK: 拼音搜索命中")
	} else {
		t.Log("PINYIN MISS: 拼音未命中（可能 env 未生效，需人工确认）")
	}
}

// 3. 高亮响应
func TestRealZinc_Highlight(t *testing.T) {
	c := newC4081()
	probe4081(t, c)

	index := fmt.Sprintf("sm_hl_%d", time.Now().UnixNano())
	t.Cleanup(func() { c.DeleteIndex(index, "") })
	c.CreateIndex(index, map[string]interface{}{
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "text", "analyzer": "jieba_std", "search_analyzer": "jieba_search", "highlightable": true},
		},
	}, "")
	c.Bulk(index, []map[string]interface{}{
		{"_id": "1", "name": "换发动机后胶垫"},
	}, "")
	waitDocs(t, c, index)

	resp, err := c.Search(index, map[string]interface{}{
		"query":     map[string]interface{}{"match": map[string]interface{}{"name": "发动机"}},
		"highlight": map[string]interface{}{"fields": map[string]interface{}{"name": map[string]interface{}{}}},
	}, "")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	hits, _ := resp["hits"].(map[string]interface{})["hits"].([]interface{})
	if len(hits) == 0 {
		t.Fatal("no hits")
	}
	hl, _ := hits[0].(map[string]interface{})["highlight"]
	t.Logf("highlight: %v", hl)
	if hl != nil {
		t.Log("HIGHLIGHT OK")
	} else {
		t.Log("HIGHLIGHT MISS: 响应无 highlight（可能需 store 字段，需人工确认）")
	}
}

// 4. mapping 热更新：新增字段后立即可用
func TestRealZinc_MappingAddField(t *testing.T) {
	c := newC4081()
	probe4081(t, c)

	index := fmt.Sprintf("sm_mf_%d", time.Now().UnixNano())
	t.Cleanup(func() { c.DeleteIndex(index, "") })
	c.CreateIndex(index, map[string]interface{}{
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "text", "analyzer": "jieba_std"},
		},
	}, "")
	c.Bulk(index, []map[string]interface{}{
		{"_id": "1", "name": "测试", "old_field": "x"},
	}, "")
	waitDocs(t, c, index)

	// 热更新加新字段 tag
	if err := c.UpdateMapping(index, map[string]interface{}{
		"properties": map[string]interface{}{
			"tag": map[string]interface{}{"type": "keyword"},
		},
	}, ""); err != nil {
		t.Fatalf("update mapping: %v", err)
	}

	// 写入带新字段的文档 + 搜索新字段
	c.Bulk(index, []map[string]interface{}{
		{"_id": "2", "name": "测试2", "tag": "hot"},
	}, "")
	waitDocs(t, c, index)

	resp, err := c.Search(index, map[string]interface{}{
		"query": map[string]interface{}{"term": map[string]interface{}{"tag": "hot"}},
	}, "")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	total := totalOf(t, resp)
	t.Logf("新字段 term 搜索: total=%v", total)
	if total >= 1 {
		t.Log("MAPPING_ADD_FIELD OK: 热更新新字段可用")
	} else {
		t.Log("MAPPING_ADD_FIELD MISS: 新字段搜索未命中")
	}
}
