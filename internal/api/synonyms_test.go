package api

import (
	"os"
	"path/filepath"
	"testing"

	"searchmiddleware/internal/config"
	"searchmiddleware/internal/metadata"
)

// TestExportSynonymsToZincFormat 验证同义词导出为 Zinc 格式（逗号分隔双向等价）
func TestExportSynonymsToZincFormat(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "synonyms.txt")

	metaDB, err := metadata.NewDB("file::memory:?cache=shared")
	if err != nil {
		t.Fatalf("meta db: %v", err)
	}
	if err := metaDB.AutoMigrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	metaDB.Create(&metadata.Synonym{Word: "手机", Synonyms: `["移动电话","handset"]`})
	metaDB.Create(&metadata.Synonym{Word: "发动机", Synonyms: `["引擎"]`})
	metaDB.Create(&metadata.Synonym{Word: "无效", Synonyms: `not-json`}) // 应被跳过

	s := &Server{
		cfg:  &config.AppConfig{Synonyms: file},
		meta: metaDB,
	}

	if err := s.exportSynonymsToZinc(); err != nil {
		// Zinc 不可达时 ReloadSynonym 会失败，但文件应已写出
		t.Logf("reload (expected fail without zinc): %v", err)
	}

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("read synonyms file: %v", err)
	}
	got := string(data)
	want := "手机,移动电话,handset\n发动机,引擎\n"
	if got != want {
		t.Errorf("export mismatch:\n got: %q\nwant: %q", got, want)
	}
}
