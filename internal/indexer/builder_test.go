package indexer

import (
	"context"
	"database/sql"
	"testing"

	"searchmiddleware/internal/config"
)

// 用 sqlmock 不可行（未引入依赖），改用内存型驱动验证纯逻辑：
// combineQueries / getPrimaryKeyColumn / convertValue 不依赖真实 DB。
func TestCombineQueries(t *testing.T) {
	b := &DocumentBuilder{}
	cases := []struct {
		base, inc, want string
	}{
		{
			"SELECT * FROM t WHERE delete_time = 0",
			"WHERE update_time > ?",
			"SELECT * FROM t WHERE delete_time = 0 AND update_time > ?",
		},
		{
			"SELECT * FROM t",
			"WHERE update_time > ?",
			"SELECT * FROM t WHERE update_time > ?",
		},
	}
	for _, c := range cases {
		got := b.combineQueries(c.base, c.inc)
		if got != c.want {
			t.Errorf("combineQueries(%q, %q) = %q, want %q", c.base, c.inc, got, c.want)
		}
	}
}

func TestGetPrimaryKeyColumn(t *testing.T) {
	b := &DocumentBuilder{}
	got := b.getPrimaryKeyColumn("SELECT maintenance_id, site_id FROM shop_maintenance WHERE x = 1")
	if got != "maintenance_id" {
		t.Errorf("pk = %q, want maintenance_id", got)
	}
	if got := b.getPrimaryKeyColumn("UPDATE t SET x=1"); got != "" {
		t.Errorf("pk = %q, want empty", got)
	}
}

func TestConvertValueArray(t *testing.T) {
	cfg := &config.IndexConfig{}
	cfg.Source.SQLFieldArray = []string{"category_names"}
	b := NewDocumentBuilder(cfg, nil)

	v := b.convertValue("引擎 机油 滤芯", "category_names")
	arr, ok := v.([]interface{})
	if !ok {
		t.Fatalf("expected []interface{}, got %T", v)
	}
	if len(arr) != 3 {
		t.Errorf("len = %d, want 3", len(arr))
	}
}

func TestConvertValueScalar(t *testing.T) {
	b := &DocumentBuilder{fields: map[string]FieldInfo{}}
	if v := b.convertValue(42, "unknown"); v != 42 {
		t.Errorf("unknown field passthrough = %v, want 42", v)
	}
	if v := b.convertValue(nil, "x"); v != nil {
		t.Errorf("nil = %v, want nil", v)
	}
}

func TestBuildByIDsEmpty(t *testing.T) {
	b := &DocumentBuilder{indexCfg: &config.IndexConfig{}, ds: nil}
	res, err := b.BuildByIDs(context.Background(), nil)
	if err != nil {
		t.Fatalf("empty ids should not error: %v", err)
	}
	if res.Count != 0 {
		t.Errorf("count = %d, want 0", res.Count)
	}
}

// 类型断言：DocumentBuilder 结构体字段可构造（无 DB 依赖）
var _ = sql.ErrNoRows
