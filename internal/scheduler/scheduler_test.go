package scheduler

import (
	"testing"

	"searchmiddleware/internal/metadata"
)

// getEntryID 修复：register 后应能取到真实 entryID（修复前恒 0）
func TestGetEntryIDAfterRegister(t *testing.T) {
	metaDB, err := metadata.NewDB("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("meta db: %v", err)
	}
	if err := metaDB.AutoMigrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sch := metadata.Schedule{IndexName: "maint", Type: "incremental", CronExpr: "*/5 * * * * *", Enabled: true}
	if err := metaDB.Create(&sch).Error; err != nil {
		t.Fatalf("create schedule: %v", err)
	}

	s := New(metaDB, nil, map[string]bool{"maint": true})
	s.register(sch)

	id := s.getEntryID(sch)
	if id == 0 {
		t.Fatal("getEntryID returned 0 (修复前恒 0 → next_run 永不更新)")
	}

	entry := s.cron.Entry(id)
	if entry.Schedule == nil {
		t.Fatalf("entry %d has no schedule", id)
	}

	// register 时已写入 next_run
	var got metadata.Schedule
	metaDB.First(&got, sch.ID)
	if got.NextRun.IsZero() {
		t.Error("next_run was not written on register")
	}
}

// 未注册的 schedule → 0（不 panic）
func TestGetEntryIDUnknownSchedule(t *testing.T) {
	metaDB, _ := metadata.NewDB("file::memory:?cache=shared")
	s := New(metaDB, nil, nil)
	if id := s.getEntryID(metadata.Schedule{ID: 999}); id != 0 {
		t.Errorf("unknown schedule: want 0, got %d", id)
	}
}
