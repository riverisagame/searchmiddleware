package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAppConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.yaml")
	content := `
security:
  jwt_secret: test-secret
zinc:
  clusters:
    default: ["http://localhost:4080"]
`
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := LoadAppConfig(path)
	if err != nil {
		t.Fatalf("LoadAppConfig: %v", err)
	}
	if cfg.Server.APIPort != 8090 {
		t.Errorf("default api port = %d, want 8090", cfg.Server.APIPort)
	}
	if cfg.Sync.BatchSize != 500 {
		t.Errorf("default batch size = %d, want 500", cfg.Sync.BatchSize)
	}
	if cfg.Env != "dev" {
		t.Errorf("default env = %s, want dev", cfg.Env)
	}
	if cfg.Zinc.Default != "default" {
		t.Errorf("default cluster = %s, want default", cfg.Zinc.Default)
	}
}

func TestLoadAppConfigMissingSecret(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.yaml")
	os.WriteFile(path, []byte("zinc:\n  clusters:\n    default: [\"http://x\"]\n"), 0644)

	if _, err := LoadAppConfig(path); err == nil {
		t.Error("expected error for missing jwt_secret")
	}
}

func TestLoadIndexConfig(t *testing.T) {
	dir := t.TempDir()
	content := `
source:
  name: s1
  datasource: main
  sql_query: "SELECT id FROM t WHERE delete_time = 0"
  incremental_field: update_time
index:
  name: myindex
  alias: true
  fields:
    name: { type: text, searchable: true }
`
	os.WriteFile(filepath.Join(dir, "myindex.yaml"), []byte(content), 0644)

	cfgs, err := LoadIndexConfig(dir)
	if err != nil {
		t.Fatalf("LoadIndexConfig: %v", err)
	}
	if len(cfgs) != 1 {
		t.Fatalf("got %d index configs, want 1", len(cfgs))
	}
	cfg := cfgs["myindex"]
	if cfg.Index.Analyzer != "jieba_std" {
		t.Errorf("default analyzer = %s, want jieba_std", cfg.Index.Analyzer)
	}
	if cfg.Source.IncrementalField != "update_time" {
		t.Errorf("incremental field = %s, want update_time", cfg.Source.IncrementalField)
	}
	if _, ok := cfg.Index.Fields["name"]; !ok {
		t.Error("field 'name' missing after parse")
	}
}

func TestLoadIndexConfigSkipsUnderscore(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "_example.yaml"), []byte("bad: [yaml"), 0644)

	cfgs, err := LoadIndexConfig(dir)
	if err != nil {
		t.Fatalf("underscore file should be skipped, got error: %v", err)
	}
	if len(cfgs) != 0 {
		t.Errorf("got %d configs, want 0", len(cfgs))
	}
}
