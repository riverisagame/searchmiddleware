package zinc

import (
	"testing"

	"searchmiddleware/internal/config"
)

// GetAlias 按 alias 名过滤：只返回含该 alias 的索引
func TestGetAliasFilterByAlias(t *testing.T) {
	srv, _ := mockZinc(t)
	client := NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {srv.URL}},
		Default:  "default",
	})

	got, err := client.GetAlias("dev_maint", "default")
	if err != nil {
		t.Fatalf("GetAlias: %v", err)
	}
	// mock 返回两个索引都含 dev_maint → 应都返回
	if len(got) != 2 {
		t.Errorf("want 2 indexes under alias, got %d: %v", len(got), got)
	}
	if _, ok := got["dev_maint_write_111"]; !ok {
		t.Error("dev_maint_write_111 should be included")
	}

	// 不存在的 alias → 空
	got2, err := client.GetAlias("no_such_alias", "default")
	if err != nil {
		t.Fatalf("GetAlias: %v", err)
	}
	if len(got2) != 0 {
		t.Errorf("want 0 for missing alias, got %d", len(got2))
	}
}
