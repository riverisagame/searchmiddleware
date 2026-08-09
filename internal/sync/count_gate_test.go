// getExpectedCount 回归验证：90% gate 依赖的 COUNT 正确性
package sync

import (
	"database/sql"
	"testing"

	_ "github.com/glebarez/sqlite"

	"searchmiddleware/internal/config"
	"searchmiddleware/internal/metadata"
)

// getExpectedCount 应返回真实行数（修复前：小写 SQL 替换失败 → 取到第一行第一列值）
func TestGetExpectedCount(t *testing.T) {
	db, err := sql.Open("sqlite", "file:countgate?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	db.SetMaxOpenConns(1) // 内存库单连接，避免连接池隔离
	if _, err := db.Exec("CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)"); err != nil {
		t.Fatalf("ddl: %v", err)
	}
	// 3 行；第一行 id=5（修复前 Scan 会取到 5 而非 3）
	for _, s := range []string{
		"INSERT INTO items VALUES (5, 'a')",
		"INSERT INTO items VALUES (6, 'b')",
		"INSERT INTO items VALUES (7, 'c')",
	} {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	metaDB, err := metadata.NewDB("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("meta: %v", err)
	}
	indexCfgs := map[string]*config.IndexConfig{
		"maint": {Source: config.IndexSourceConfig{
			DataSource: "d1",
			SQLQuery:   "select id, name from items", // 小写（真实 SQL 常态）
		}},
	}
	e := &Engine{
		cfg:       &config.AppConfig{},
		indexCfgs: indexCfgs,
		metadata:  metaDB,
		dsMap:     map[string]*sql.DB{"d1": db},
	}

	got := e.getExpectedCount("maint")
	if got != 3 {
		t.Errorf("getExpectedCount: want 3, got %d (修复前会取到第一行 id=5 → 90%% gate 误判)", got)
	}

	// 大写 SQL 也应正确
	indexCfgs["maint"].Source.SQLQuery = "SELECT id, name FROM items"
	got2 := e.getExpectedCount("maint")
	if got2 != 3 {
		t.Errorf("uppercase sql: want 3, got %d", got2)
	}

	// 带 WHERE 的 SQL（增量字段过滤场景的 count 保持 WHERE）
	indexCfgs["maint"].Source.SQLQuery = "select id, name from items where status = 1"
	got3 := e.getExpectedCount("maint")
	if got3 != 0 {
		t.Errorf("where sql: want 0 (no rows match), got %d", got3)
	}
}
