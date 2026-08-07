package sync

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"searchmiddleware/internal/config"
	"searchmiddleware/internal/lifecycle"
	"searchmiddleware/internal/metadata"
	"searchmiddleware/internal/zinc"
)

func mockZincServer(t *testing.T) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/healthz":
			w.Write([]byte(`{"status":"ok"}`))
		case len(r.URL.Path) > 8 && r.URL.Path[len(r.URL.Path)-8:] == "/_search":
			// 模拟 42 个文档
			w.Write([]byte(`{"hits":{"total":{"value":42},"hits":[]}}`))
		default:
			w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newTestEngine(t *testing.T, zincURL string) *Engine {
	appCfg := &config.AppConfig{
		Env: "dev",
		Zinc: config.ZincConfig{
			Clusters: map[string][]string{"default": {zincURL}},
			Default:  "default",
		},
	}

	indexCfg := &config.IndexConfig{
		Source: config.IndexSourceConfig{
			Name:       "s1",
			DataSource: "main",
			SQLQuery:   "SELECT maintenance_id, name FROM shop_maintenance WHERE delete_time = 0",
		},
		Index: config.IndexIndexConfig{
			Name: "maintenance",
		},
	}
	indexCfgs := map[string]*config.IndexConfig{"maintenance": indexCfg}

	metaDB, err := metadata.NewDB("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("meta db: %v", err)
	}
	if err := metaDB.AutoMigrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	zc := zinc.NewClient(&appCfg.Zinc)
	lm := lifecycle.NewManager(appCfg, indexCfgs, metaDB, zc)

	return NewEngine(appCfg, indexCfgs, metaDB, zc, lm, nil)
}

func TestReconcileCountIntegration(t *testing.T) {
	srv := mockZincServer(t)
	e := newTestEngine(t, srv.URL)

	result, err := e.ReconcileCount("maintenance")
	if err != nil {
		t.Fatalf("reconcile count: %v", err)
	}
	if result.IndexCount != 42 {
		t.Errorf("IndexCount = %d, want 42", result.IndexCount)
	}
	if result.Type != "count" {
		t.Errorf("Type = %s, want count", result.Type)
	}

	// 验证落库
	var saved []metadata.ReconcileResult
	e.metadata.Find(&saved)
	if len(saved) != 1 {
		t.Fatalf("saved results = %d, want 1", len(saved))
	}
	if saved[0].IndexName != "maintenance" {
		t.Errorf("saved index = %s, want maintenance", saved[0].IndexName)
	}
}

func TestReconcileUnknownIndex(t *testing.T) {
	e := newTestEngine(t, "http://127.0.0.1:1")
	if _, err := e.ReconcileCount("nope"); err == nil {
		t.Error("unknown index should error")
	}
}

var _ = json.Marshal
