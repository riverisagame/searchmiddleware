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
	ID        uint   `gorm:"primarykey"`
	Username  string `gorm:"uniqueIndex;size:64;not null"`
	Password  string `gorm:"size:128;not null"`
	Role      string `gorm:"size:16;not null;default:viewer"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type SyncCursor struct {
	IndexName   string `gorm:"primarykey;size:128"`
	CursorValue string `gorm:"type:text"`
	UpdatedAt   time.Time
}

type SyncRun struct {
	ID          uint   `gorm:"primarykey"`
	IndexName   string `gorm:"size:128;index"`
	Type        string `gorm:"size:16;index"`
	Trigger     string `gorm:"size:16"`
	Status      string `gorm:"size:16;index"`
	RowsCount   int64
	DurationMs  int64
	Throughput  float64
	ErrorCount  int
	StartedAt   time.Time `gorm:"index"`
	CompletedAt *time.Time
}

type SyncLog struct {
	ID         uint   `gorm:"primarykey"`
	RunID      uint   `gorm:"index"`
	IndexName  string `gorm:"size:128;index"`
	Level      string `gorm:"size:16"`
	Message    string `gorm:"type:text"`
	Task       string `gorm:"size:32"`
	DurationMs int64
	CreatedAt  time.Time `gorm:"index"`
}

type Schedule struct {
	ID        uint   `gorm:"primarykey"`
	IndexName string `gorm:"size:128;index"`
	Type      string `gorm:"size:16"`
	CronExpr  string `gorm:"size:64"`
	Enabled   bool   `gorm:"default:true"`
	LastRun   *time.Time
	NextRun   *time.Time `gorm:"index"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ReconcileResult struct {
	ID           uint   `gorm:"primarykey"`
	IndexName    string `gorm:"size:128;index"`
	Type         string `gorm:"size:16"`
	IndexCount   int64
	DBValidCount int64
	MissingCount int64
	ExtraCount   int64
	MissingIDs   string    `gorm:"type:text"`
	ExtraIDs     string    `gorm:"type:text"`
	Status       string    `gorm:"size:16"`
	CreatedAt    time.Time `gorm:"index"`
}

type SyncAlert struct {
	ID        uint      `gorm:"primarykey"`
	IndexName string    `gorm:"size:128;index"`
	Level     string    `gorm:"size:16"`
	Message   string    `gorm:"type:text"`
	Read      bool      `gorm:"default:false"`
	CreatedAt time.Time `gorm:"index"`
}

type Synonym struct {
	ID        uint   `gorm:"primarykey"`
	Word      string `gorm:"size:128;index"`
	Synonyms  string `gorm:"type:text"`
	Indexes   string `gorm:"size:512"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type IndexConfig struct {
	ID        uint   `gorm:"primarykey"`
	Name      string `gorm:"size:128;uniqueIndex"`
	Config    string `gorm:"type:text"`
	Version   string `gorm:"size:64"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
