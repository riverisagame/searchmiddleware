package metadata

import (
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DB struct {
	*gorm.DB
}

func NewDB(dsn string) (*DB, error) {
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return &DB{db}, nil
}

func (d *DB) AutoMigrate() error {
	return d.DB.AutoMigrate(
		&User{},
		&SyncCursor{},
		&SyncRun{},
		&SyncLog{},
		&Schedule{},
		&ReconcileResult{},
		&SyncAlert{},
		&Synonym{},
		&IndexConfig{},
	)
}

type User struct {
	ID        uint   `json:"id" gorm:"primarykey"`
	Username  string `json:"username" gorm:"uniqueIndex;size:64;not null"`
	Password  string `json:"-" gorm:"size:128;not null"`
	Role      string `json:"role" gorm:"size:16;not null;default:viewer"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SyncCursor struct {
	IndexName   string `json:"index_name" gorm:"primarykey;size:128"`
	CursorValue string `json:"cursor_value" gorm:"type:text"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type SyncRun struct {
	ID          uint   `json:"id" gorm:"primarykey"`
	IndexName   string `json:"index_name" gorm:"size:128;index"`
	Type        string `json:"type" gorm:"size:16;index"`
	Trigger     string `json:"trigger" gorm:"size:16"`
	Status      string `json:"status" gorm:"size:16;index"`
	RowsCount   int64  `json:"rows_count"`
	DurationMs  int64  `json:"duration_ms"`
	Throughput  float64 `json:"throughput"`
	ErrorCount  int    `json:"error_count"`
	StartedAt   time.Time `json:"started_at" gorm:"index"`
	CompletedAt *time.Time `json:"completed_at"`
}

type SyncLog struct {
	ID         uint   `json:"id" gorm:"primarykey"`
	RunID      uint   `json:"run_id" gorm:"index"`
	IndexName  string `json:"index_name" gorm:"size:128;index"`
	Level      string `json:"level" gorm:"size:16"`
	Message    string `json:"message" gorm:"type:text"`
	Task       string `json:"task" gorm:"size:32"`
	DurationMs int64  `json:"duration_ms"`
	CreatedAt  time.Time `json:"created_at" gorm:"index"`
}

type Schedule struct {
	ID        uint   `json:"id" gorm:"primarykey"`
	IndexName string `json:"index_name" gorm:"size:128;index"`
	Type      string `json:"type" gorm:"size:16"`
	CronExpr  string `json:"cron_expr" gorm:"size:64"`
	Enabled   bool   `json:"enabled" gorm:"default:true"`
	LastRun   *time.Time `json:"last_run"`
	NextRun   *time.Time `json:"next_run" gorm:"index"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type ReconcileResult struct {
	ID           uint   `json:"id" gorm:"primarykey"`
	IndexName    string `json:"index_name" gorm:"size:128;index"`
	Type         string `json:"type" gorm:"size:16"`
	IndexCount   int64  `json:"index_count"`
	DBValidCount int64  `json:"db_valid_count"`
	MissingCount int64  `json:"missing_count"`
	ExtraCount   int64  `json:"extra_count"`
	MissingIDs   string `json:"missing_ids" gorm:"type:text"`
	ExtraIDs     string `json:"extra_ids" gorm:"type:text"`
	Status       string `json:"status" gorm:"size:16"`
	CreatedAt    time.Time `json:"created_at" gorm:"index"`
}

type SyncAlert struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	IndexName string    `json:"index_name" gorm:"size:128;index"`
	Level     string    `json:"level" gorm:"size:16"`
	Message   string    `json:"message" gorm:"type:text"`
	Read      bool      `json:"read" gorm:"default:false"`
	CreatedAt time.Time `json:"created_at" gorm:"index"`
}

type Synonym struct {
	ID        uint   `json:"id" gorm:"primarykey"`
	Word      string `json:"word" gorm:"size:128;index"`
	Synonyms  string `json:"synonyms" gorm:"type:text"`
	Indexes   string `json:"indexes" gorm:"size:512"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type IndexConfig struct {
	ID        uint   `json:"id" gorm:"primarykey"`
	Name      string `json:"name" gorm:"size:128;uniqueIndex"`
	Config    string `json:"config" gorm:"type:text"`
	Version   string `json:"version" gorm:"size:64"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
