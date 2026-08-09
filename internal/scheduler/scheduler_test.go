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

func newTestScheduler(t *testing.T) (*Scheduler, *metadata.DB) {
	metaDB, err := metadata.NewDB("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("meta db: %v", err)
	}
	if err := metaDB.AutoMigrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return New(metaDB, nil, nil), metaDB
}

func entryCount(s *Scheduler) int { return len(s.cron.Entries()) }

// 启停：Toggle(false) 移除 cron entry，Toggle(true) 恢复
func TestToggleScheduleSyncsCron(t *testing.T) {
	s, metaDB := newTestScheduler(t)
	sch := metadata.Schedule{IndexName: "maint", Type: "incremental", CronExpr: "*/5 * * * * *", Enabled: true}
	if err := metaDB.Create(&sch).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	s.register(sch)
	if n := entryCount(s); n != 1 {
		t.Fatalf("after register: want 1 entry, got %d", n)
	}

	// 停用 → entry 移除
	if err := s.ToggleSchedule(sch.ID, false); err != nil {
		t.Fatalf("toggle off: %v", err)
	}
	if n := entryCount(s); n != 0 {
		t.Errorf("after disable: want 0 entries, got %d", n)
	}
	if id := s.getEntryID(sch); id != 0 {
		t.Errorf("disabled schedule should have no entry id, got %d", id)
	}

	// 启用 → entry 恢复
	if err := s.ToggleSchedule(sch.ID, true); err != nil {
		t.Fatalf("toggle on: %v", err)
	}
	if n := entryCount(s); n != 1 {
		t.Errorf("after enable: want 1 entry, got %d", n)
	}
}

// 删除：DB 删除 + cron entry 移除
func TestRemoveScheduleSyncsCron(t *testing.T) {
	s, metaDB := newTestScheduler(t)
	sch := metadata.Schedule{IndexName: "maint", Type: "full", CronExpr: "0 0 2 * * *", Enabled: true}
	if err := metaDB.Create(&sch).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	s.register(sch)

	if err := s.RemoveSchedule(sch.ID); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if n := entryCount(s); n != 0 {
		t.Errorf("after remove: want 0 entries, got %d", n)
	}
	var count int64
	metaDB.Model(&metadata.Schedule{}).Where("id = ?", sch.ID).Count(&count)
	if count != 0 {
		t.Error("schedule row should be deleted from DB")
	}
}

// 重复注册幂等（更新场景）：不产生重复 entry
func TestRegisterIdempotent(t *testing.T) {
	s, metaDB := newTestScheduler(t)
	sch := metadata.Schedule{IndexName: "maint", Type: "incremental", CronExpr: "*/5 * * * * *", Enabled: true}
	metaDB.Create(&sch)
	s.register(sch)
	s.register(sch) // 同 id 重复注册
	if n := entryCount(s); n != 1 {
		t.Errorf("re-register: want 1 entry, got %d", n)
	}
}
