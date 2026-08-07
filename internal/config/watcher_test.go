package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWatcherDetectsYamlChange(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "goods.yaml")
	os.WriteFile(path, []byte("source:\n  sql_query: \"SELECT 1\"\nindex:\n  name: goods\n"), 0644)

	changed := make(chan string, 4)
	w, err := NewWatcher(dir, func(name string) {
		changed <- name
	})
	if err != nil {
		t.Fatalf("new watcher: %v", err)
	}
	w.Start()
	defer w.Stop()

	// 模拟原子写（rename 覆盖，同 Q4 的 SaveIndexConfig 模式）
	os.WriteFile(filepath.Join(dir, "goods.yaml.tmp"), []byte("updated"), 0644)
	os.Rename(filepath.Join(dir, "goods.yaml.tmp"), path)

	select {
	case name := <-changed:
		if name != "goods" {
			t.Errorf("changed name = %s, want goods", name)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for change event")
	}
}

func TestWatcherIgnoresNonYaml(t *testing.T) {
	dir := t.TempDir()
	changed := make(chan string, 4)
	w, err := NewWatcher(dir, func(name string) {
		changed <- name
	})
	if err != nil {
		t.Fatalf("new watcher: %v", err)
	}
	w.Start()
	defer w.Stop()

	// 非 yaml 文件不应触发
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0644)

	select {
	case name := <-changed:
		t.Errorf("non-yaml file should not trigger, got %s", name)
	case <-time.After(2 * time.Second):
		// 正确：无事件
	}
}
