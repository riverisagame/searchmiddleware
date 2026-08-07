package metadata

import (
	"testing"
)

func newTestDB(t *testing.T) *DB {
	db, err := NewDB("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, _ := db.DB.DB()
		sqlDB.Close()
	})
	return db
}

func TestUserCRUD(t *testing.T) {
	db := newTestDB(t)

	u := User{Username: "admin", Password: "hash", Role: "admin"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	var got User
	if err := db.Where("username = ?", "admin").First(&got).Error; err != nil {
		t.Fatalf("find: %v", err)
	}
	if got.Role != "admin" {
		t.Errorf("role = %s, want admin", got.Role)
	}

	// 唯一索引
	dup := User{Username: "admin", Password: "x", Role: "viewer"}
	if err := db.Create(&dup).Error; err == nil {
		t.Error("duplicate username should fail")
	}
}

func TestSyncCursorUpsert(t *testing.T) {
	db := newTestDB(t)

	db.Exec("INSERT OR REPLACE INTO sync_cursors (index_name, cursor_value, updated_at) VALUES ('maintenance', '100', datetime('now'))")

	var c SyncCursor
	if err := db.Where("index_name = ?", "maintenance").First(&c).Error; err != nil {
		t.Fatalf("find cursor: %v", err)
	}
	if c.CursorValue != "100" {
		t.Errorf("cursor = %s, want 100", c.CursorValue)
	}
}

func TestSynonymAndAlert(t *testing.T) {
	db := newTestDB(t)

	db.Create(&Synonym{Word: "手机", Synonyms: `["移动电话","handset"]`, Indexes: "maintenance"})
	db.Create(&SyncAlert{IndexName: "maintenance", Level: "ERROR", Message: "boom"})

	var syns []Synonym
	db.Find(&syns)
	if len(syns) != 1 {
		t.Errorf("synonyms = %d, want 1", len(syns))
	}

	var alerts []SyncAlert
	db.Find(&alerts)
	if len(alerts) != 1 {
		t.Errorf("alerts = %d, want 1", len(alerts))
	}
}

func TestScheduleCRUD(t *testing.T) {
	db := newTestDB(t)

	s := Schedule{IndexName: "maintenance", Type: "incremental", CronExpr: "*/5 * * * * *", Enabled: true}
	if err := db.Create(&s).Error; err != nil {
		t.Fatalf("create: %v", err)
	}

	db.Model(&Schedule{}).Where("id = ?", s.ID).Update("enabled", false)

	var got Schedule
	db.First(&got, s.ID)
	if got.Enabled {
		t.Error("enabled should be false after update")
	}
}
