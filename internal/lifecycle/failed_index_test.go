package lifecycle

import (
	"testing"

	"searchmiddleware/internal/config"
	"searchmiddleware/internal/metadata"
	"searchmiddleware/internal/zinc"
)

// failed_ 前缀的 write 索引应视为无效（触发下次全量重建）——bulk 失败后不永久卡死
func TestGetWriteIndexFailedPrefix(t *testing.T) {
	metaDB, err := metadata.NewDB("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("meta: %v", err)
	}
	if err := metaDB.AutoMigrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	metaDB.Create(&metadata.IndexConfig{Name: "maint", Config: "dev_maint_write_111"})

	mgr := NewManager(&config.AppConfig{Env: "dev"}, map[string]*config.IndexConfig{}, metaDB, &zinc.Client{})

	// 正常：返回记录
	if got := mgr.GetWriteIndex("maint"); got != "dev_maint_write_111" {
		t.Errorf("normal: want dev_maint_write_111, got %q", got)
	}

	// 标记失败后：返回空（触发重建）
	mgr.MarkWriteIndexFailed("maint", "dev_maint_write_111")
	if got := mgr.GetWriteIndex("maint"); got != "" {
		t.Errorf("after fail: want empty, got %q", got)
	}
}
