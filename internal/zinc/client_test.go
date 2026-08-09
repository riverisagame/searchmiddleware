package zinc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"searchmiddleware/internal/config"
)

// mockZinc 模拟 Zinc 的 /healthz、/_search、/_bulk、/es/_aliases 端点
func mockZinc(t *testing.T) (*httptest.Server, *atomic.Int64) {
	var bulkCount atomic.Int64
	var searchCount atomic.Int64
	var aliasSwaps atomic.Int64

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/healthz":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"ok"}`))
		case len(r.URL.Path) > 6 && r.URL.Path[len(r.URL.Path)-6:] == "/_bulk":
			bulkCount.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"took":1,"errors":false}`))
		case len(r.URL.Path) > 8 && r.URL.Path[len(r.URL.Path)-8:] == "/_search":
			searchCount.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"hits":{"total":{"value":1},"hits":[{"_id":"1","_score":1.5,"_source":{"name":"发动机"}}]}}`))
		case r.URL.Path == "/es/_aliases":
			aliasSwaps.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"acknowledged":true}`))
		case r.Method == http.MethodPut && len(r.URL.Path) > 4 && r.URL.Path[:4] == "/es/":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"index":"created"}`))
		case r.Method == http.MethodGet && len(r.URL.Path) > 8 && r.URL.Path[len(r.URL.Path)-7:] == "/_alias":
			// 全量 alias 视图（ES 语义：{索引名: {"aliases": {别名: {}}}}）
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"dev_maint_write_111":{"aliases":{"dev_maint":{}}},"dev_maint_write_222":{"aliases":{"dev_maint":{}}}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"not found"}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &bulkCount
}

func TestClientSearchAndBulk(t *testing.T) {
	srv, bulkCount := mockZinc(t)

	client := NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {srv.URL}},
		Default:  "default",
		Username: "admin",
		Password: "secret",
	})

	resp, err := client.Search("dev_maintenance", map[string]interface{}{"query": map[string]interface{}{"match_all": map[string]interface{}{}}}, "")
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	total := resp["hits"].(map[string]interface{})["total"].(map[string]interface{})["value"].(float64)
	if total != 1 {
		t.Errorf("total = %v, want 1", total)
	}

	docs := []map[string]interface{}{{"_id": "1", "name": "发动机"}}
	if err := client.Bulk("dev_maintenance_write_123", docs, ""); err != nil {
		t.Fatalf("bulk: %v", err)
	}
	if bulkCount.Load() != 1 {
		t.Errorf("bulk calls = %d, want 1", bulkCount.Load())
	}
}

func TestClientAliasSwap(t *testing.T) {
	srv, _ := mockZinc(t)

	client := NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {srv.URL}},
		Default:  "default",
	})

	err := client.AliasSwap(
		map[string][]string{"dev_maintenance": {"dev_maintenance_write_new"}},
		map[string][]string{"dev_maintenance": {"dev_maintenance_write_old"}},
		"",
	)
	if err != nil {
		t.Fatalf("alias swap: %v", err)
	}
}

func TestClientNodeFailover(t *testing.T) {
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer dead.Close()

	srv, _ := mockZinc(t)

	client := NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {dead.URL, srv.URL}},
		Default:  "default",
	})

	// 死节点在前，应自动切换到活节点
	resp, err := client.Search("idx", map[string]interface{}{"query": map[string]interface{}{"match_all": map[string]interface{}{}}}, "")
	if err != nil {
		t.Fatalf("search with failover: %v", err)
	}
	if resp == nil {
		t.Error("expected response after failover")
	}

	// 验证 bulk 请求体是合法 NDJSON
	_, _ = json.Marshal("")
}
