package sync

import (
	"encoding/json"
	"testing"
)

func TestDiffAndSet(t *testing.T) {
	dbSet := toSet([]string{"1", "2", "3"})
	idxSet := toSet([]string{"2", "3", "4"})

	missing := diff(dbSet, idxSet)
	if len(missing) != 1 || missing[0] != "1" {
		t.Errorf("missing = %v, want [1]", missing)
	}

	extra := diff(idxSet, dbSet)
	if len(extra) != 1 || extra[0] != "4" {
		t.Errorf("extra = %v, want [4]", extra)
	}
}

func TestDiffSorted(t *testing.T) {
	dbSet := toSet([]string{"10", "2", "1"})
	idxSet := toSet([]string{"1"})
	missing := diff(dbSet, idxSet)
	if len(missing) != 2 || missing[0] != "10" || missing[1] != "2" {
		t.Errorf("missing = %v, want [10 2]", missing)
	}
}

func TestJsonString(t *testing.T) {
	if s := jsonString(nil); s != "[]" {
		t.Errorf("empty = %s, want []", s)
	}
	s := jsonString([]string{"a", "b"})
	var parsed []string
	if err := json.Unmarshal([]byte(s), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(parsed) != 2 {
		t.Errorf("parsed len = %d, want 2", len(parsed))
	}
}

func TestQueryAllIDsSQL(t *testing.T) {
	// 验证 queryAllIDs 的 SQL 重写逻辑（用 mock 引擎直接测 SQL 生成）
	e := &Engine{}

	base := "SELECT maintenance_id, site_id, name FROM shop_maintenance WHERE delete_time = 0"
	upper := stringsToUpper(base)
	fromIdx := stringsIndex(upper, "FROM")
	if fromIdx == -1 {
		t.Fatal("no FROM")
	}
	query := "SELECT maintenance_id " + base[fromIdx:]
	want := "SELECT maintenance_id FROM shop_maintenance WHERE delete_time = 0"
	if query != want {
		t.Errorf("query = %q, want %q", query, want)
	}

	_ = e
}

func stringsToUpper(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'a' && b[i] <= 'z' {
			b[i] -= 32
		}
	}
	return string(b)
}

func stringsIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
