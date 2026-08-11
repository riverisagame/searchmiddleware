package lifecycle

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"searchmiddleware/internal/config"
	"searchmiddleware/internal/metadata"
	"searchmiddleware/internal/zinc"
)

// 回归（P0）：CreateWriteIndex 每次返回全新索引名（full sync 绝不复用陈旧 write 索引）
func TestCreateWriteIndexTwiceDistinct(t *testing.T) {
	var mu sync.Mutex
	var puts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		puts = append(puts, r.Method+" "+r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"acknowledged":true}`))
	}))
	defer srv.Close()

	metaDB, err := metadata.NewDB("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("meta: %v", err)
	}
	if err := metaDB.AutoMigrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	zc := zinc.NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {srv.URL}},
		Default:  "default",
	})
	cfgs := map[string]*config.IndexConfig{
		"maint": {Index: config.IndexIndexConfig{ZincCluster: "default"}},
	}
	mgr := NewManager(&config.AppConfig{Env: "dev"}, cfgs, metaDB, zc)

	a, err := mgr.CreateWriteIndex("maint")
	if err != nil {
		t.Fatalf("create 1: %v", err)
	}
	b, err := mgr.CreateWriteIndex("maint")
	if err != nil {
		t.Fatalf("create 2: %v", err)
	}
	if a == "" || b == "" {
		t.Fatalf("empty write index: %q %q", a, b)
	}
	if a == b {
		t.Errorf("P0 regression: CreateWriteIndex returned same index twice (%q) - full rebuild would reuse stale index", a)
	}
	if got := mgr.GetWriteIndex("maint"); got != b {
		t.Errorf("GetWriteIndex after upsert: want %q, got %q", b, got)
	}
	putCount := 0
	for _, p := range puts {
		if strings.HasPrefix(p, "PUT ") {
			putCount++
		}
	}
	if putCount != 2 {
		t.Errorf("expected 2 PUT create calls, got %d: %v", putCount, puts)
	}
}

// 回归（P0）：WriteIndex 列优先于 Config 列（reload 覆盖 Config 后仍返回正确 write 索引）
func TestGetWriteIndexPrefersColumn(t *testing.T) {
	metaDB, err := metadata.NewDB("file:prefercol?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("meta: %v", err)
	}
	if err := metaDB.AutoMigrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// 模拟 reload 后状态：Config=YAML，WriteIndex=真实索引
	metaDB.Create(&metadata.IndexConfig{
		Name:       "maint",
		Config:     fmt.Sprintf("source:\n    datasource: main\nindex:\n    name: maint"),
		WriteIndex: "dev_maint_write_222",
	})
	mgr := NewManager(&config.AppConfig{Env: "dev"}, map[string]*config.IndexConfig{}, metaDB, &zinc.Client{})
	if got := mgr.GetWriteIndex("maint"); got != "dev_maint_write_222" {
		t.Errorf("want dev_maint_write_222, got %q", got)
	}
}

var _ = strings.Contains
