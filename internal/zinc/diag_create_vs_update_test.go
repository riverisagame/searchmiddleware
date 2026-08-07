package zinc

import (
	"fmt"
	"testing"
	"time"

	"searchmiddleware/internal/config"
)

// TestRealZinc_DiagCreateVsUpdate 区分 CreateIndex 与 UpdateMapping 的 boost 生效路径
func TestRealZinc_DiagCreateVsUpdate(t *testing.T) {
	if !pingRealZinc(t) {
		t.Skip("real zinc not available")
	}
	client := NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {"http://localhost:4080"}},
		Default:  "default",
		Username: "admin",
		Password: "Complexpass#123",
	})

	index := fmt.Sprintf("sm_cvu_%d", time.Now().UnixNano())
	t.Cleanup(func() { client.DeleteIndex(index, "") })

	// 1. CreateIndex 带 boost=10
	if err := client.CreateIndex(index, map[string]interface{}{
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "text", "boost": 10.0, "analyzer": "jieba_std"},
		},
	}, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	client.Bulk(index, []map[string]interface{}{{"_id": "1", "name": "换发动机后胶垫"}}, "")
	time.Sleep(1500 * time.Millisecond)
	scoreCreate := scoreOf(t, client, index, map[string]interface{}{
		"match": map[string]interface{}{"name": "发动机"},
	})
	t.Logf("CreateIndex 后 match 分数: %.4f", scoreCreate)

	// 2. UpdateMapping 重新设置 boost=10（热更新路径）
	if err := client.UpdateMapping(index, map[string]interface{}{
		"properties": map[string]interface{}{
			"name": map[string]interface{}{"type": "text", "boost": 10.0, "analyzer": "jieba_std"},
		},
	}, ""); err != nil {
		t.Fatalf("update: %v", err)
	}
	time.Sleep(500 * time.Millisecond)
	scoreUpdate := scoreOf(t, client, index, map[string]interface{}{
		"match": map[string]interface{}{"name": "发动机"},
	})
	t.Logf("UpdateMapping 后 match 分数: %.4f", scoreUpdate)

	if scoreUpdate > scoreCreate*1.5 {
		t.Logf("结论: CreateIndex 路径丢 Boost（%.4f），UpdateMapping 路径生效（%.4f）→ DeepClone 丢 Boost 且 CreateIndex 走 DeepClone", scoreCreate, scoreUpdate)
	} else {
		t.Logf("结论: 两条路径分数一致（%.4f vs %.4f）→ 问题在别处", scoreCreate, scoreUpdate)
	}
}
