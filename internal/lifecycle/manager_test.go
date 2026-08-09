package lifecycle

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"searchmiddleware/internal/config"
	"searchmiddleware/internal/metadata"
	"searchmiddleware/internal/zinc"
)

// SwitchAlias 应正确构造 removeMap：从旧索引移除 readAlias，仅保留新索引
func TestSwitchAliasRemovesOldIndex(t *testing.T) {
	// mock zinc：记录 AliasSwap 请求体
	var lastSwapBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/healthz":
			w.Write([]byte(`{"status":"ok"}`))
		case r.URL.Path == "/es/_alias":
			// 旧索引 dev_maint_write_111 持有 readAlias dev_maint
			w.Write([]byte(`{"dev_maint_write_111":{"aliases":{"dev_maint":{}}}}`))
		case r.URL.Path == "/es/_aliases":
			b, _ := io.ReadAll(r.Body)
			lastSwapBody = string(b)
			w.Write([]byte(`{"acknowledged":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	zClient := zinc.NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {srv.URL}},
		Default:  "default",
	})

	metaDB, err := metadata.NewDB("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("meta db: %v", err)
	}

	mgr := NewManager(&config.AppConfig{Env: "dev"}, map[string]*config.IndexConfig{
		"maint": {Index: config.IndexIndexConfig{ZincCluster: "default"}},
	}, metaDB, zClient)

	if err := mgr.SwitchAlias("maint", "dev_maint_write_222"); err != nil {
		t.Fatalf("SwitchAlias: %v", err)
	}

	// 断言 AliasSwap 请求体
	if lastSwapBody == "" {
		t.Fatal("AliasSwap was not called")
	}
	var body struct {
		Actions []map[string]map[string]string `json:"actions"`
	}
	if err := json.Unmarshal([]byte(lastSwapBody), &body); err != nil {
		t.Fatalf("parse swap body: %v (%s)", err, lastSwapBody)
	}
	var adds, removes []string
	for _, a := range body.Actions {
		if add, ok := a["add"]; ok {
			adds = append(adds, add["index"]+"/"+add["alias"])
		}
		if rm, ok := a["remove"]; ok {
			removes = append(removes, rm["index"]+"/"+rm["alias"])
		}
	}
	if len(adds) != 1 || adds[0] != "dev_maint_write_222/dev_maint" {
		t.Errorf("add action: want dev_maint_write_222/dev_maint, got %v", adds)
	}
	if len(removes) != 1 || removes[0] != "dev_maint_write_111/dev_maint" {
		t.Errorf("remove action: want dev_maint_write_111/dev_maint, got %v", removes)
	}
}

// 无旧索引时（首次切换）：仅 add，无 remove
func TestSwitchAliasFirstTimeNoRemove(t *testing.T) {
	var lastSwapBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/healthz":
			w.Write([]byte(`{"status":"ok"}`))
		case r.URL.Path == "/es/_alias":
			w.Write([]byte(`{}`)) // 无旧 alias
		case r.URL.Path == "/es/_aliases":
			b, _ := io.ReadAll(r.Body)
			lastSwapBody = string(b)
			w.Write([]byte(`{"acknowledged":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{}`))
		}
	}))
	defer srv.Close()

	zClient := zinc.NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {srv.URL}},
		Default:  "default",
	})
	metaDB, _ := metadata.NewDB("file::memory:?cache=shared")
	mgr := NewManager(&config.AppConfig{Env: "dev"}, map[string]*config.IndexConfig{
		"maint": {Index: config.IndexIndexConfig{ZincCluster: "default"}},
	}, metaDB, zClient)

	if err := mgr.SwitchAlias("maint", "dev_maint_write_222"); err != nil {
		t.Fatalf("SwitchAlias: %v", err)
	}
	if strings.Contains(lastSwapBody, "remove") {
		t.Errorf("first switch should not remove: %s", lastSwapBody)
	}
	if !strings.Contains(lastSwapBody, "dev_maint_write_222") {
		t.Errorf("should add new index: %s", lastSwapBody)
	}
}
