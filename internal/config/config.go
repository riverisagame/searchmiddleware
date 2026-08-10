package config

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type AppConfig struct {
	Server    ServerConfig   `yaml:"server"`
	RateLimit RateLimitConfig `yaml:"rate_limit"`
	Security  SecurityConfig `yaml:"security"`
	Zinc      ZincConfig     `yaml:"zinc"`
	Sync      SyncConfig     `yaml:"sync"`
	Storage   string         `yaml:"storage"`
	Env       string         `yaml:"env"`
	DataDir   string         `yaml:"data_dir"`
	Synonyms  string         `yaml:"synonyms_file"` // ͬ��ʵ����ļ���Zinc SynonymPath ��������
	LogLevel  string         `yaml:"log_level"`     // debug/info/warn/error（Q39 级别控制）
}

type ServerConfig struct {
	APIPort     int    `yaml:"api_port"`
	GUIPort     int    `yaml:"gui_port"`
	ReadTimeout string `yaml:"read_timeout"`
}

// RateLimitConfig 搜索 API 限流（Q39 429 观测配套，roadmap 12）
type RateLimitConfig struct {
	Enabled bool `yaml:"enabled"`
	QPS     int  `yaml:"qps"` // 每 IP 每秒请求上限（固定窗口）
}

type SecurityConfig struct {
	JWTSecret   string `yaml:"jwt_secret"`
	TokenExpiry string `yaml:"token_expiry"`
}

type ZincConfig struct {
	Clusters map[string][]string `yaml:"clusters"`
	Default  string              `yaml:"default"`
	Username string              `yaml:"username"`
	Password string              `yaml:"password"`
}

type SyncConfig struct {
	BatchSize           int    `yaml:"batch_size"`
	MaxParallelIndexes  int    `yaml:"max_parallel_indexes"`
	QueryTimeout        string `yaml:"query_timeout"`
	IncrementalInterval string `yaml:"incremental_interval"`
	FullRebuildTime     string `yaml:"full_rebuild_time"`
}

type DataSourceConfig struct {
	Name     string `yaml:"name"`
	DSN      string `yaml:"dsn"`
	ReadDSN  string `yaml:"read_dsn"`
	MaxOpen  int    `yaml:"max_open"`
	MaxIdle  int    `yaml:"max_idle"`
	ReadOnly bool   `yaml:"read_only"`
}

type IndexConfig struct {
	Source IndexSourceConfig `yaml:"source"`
	Index  IndexIndexConfig  `yaml:"index"`
}

type IndexSourceConfig struct {
	Name             string            `yaml:"name"`
	DataSource       string            `yaml:"datasource"`
	SQLQuery         string            `yaml:"sql_query"`
	SQLAttrUint      []string          `yaml:"sql_attr_uint"`
	SQLAttrFloat     []string          `yaml:"sql_attr_float"`
	SQLAttrTimestamp []string          `yaml:"sql_attr_timestamp"`
	SQLAttrKeyword   []string          `yaml:"sql_attr_keyword"`
	SQLAttrArray     []string          `yaml:"sql_attr_array"`
	SQLFieldText     []string          `yaml:"sql_field_text"`
	SQLFieldKeyword  []string          `yaml:"sql_field_keyword"`
	SQLFieldArray    []string          `yaml:"sql_field_array"`
	SQLJoinedField   map[string]string `yaml:"sql_joined_field"`
	SQLIncremental   string            `yaml:"sql_incremental"`
	IncrementalField string            `yaml:"incremental_field"`
}

type IndexIndexConfig struct {
	Name           string                    `yaml:"name"`
	Source         string                    `yaml:"source"`
	Alias          bool                      `yaml:"alias"`
	DataSource     string                    `yaml:"datasource"`
	Analyzers      map[string]AnalyzerConfig `yaml:"analyzers"`
	Analyzer       string                    `yaml:"analyzer"`
	SearchAnalyzer string                    `yaml:"search_analyzer"`
	MinWordLen     int                       `yaml:"min_word_len"`
	Boost          map[string]float64        `yaml:"boost"`
	Fields         map[string]FieldConfig    `yaml:"fields"`
	MappingOptions MappingOptionsConfig      `yaml:"mapping_options"`
	Dictionary     DictionaryConfig          `yaml:"dictionary"`
	ZincCluster    string                    `yaml:"zinc_cluster"`
}

type AnalyzerConfig struct {
	Type   string `yaml:"type"`
	Search bool   `yaml:"search"`
}

type FieldConfig struct {
	Type           string                 `yaml:"type"`
	Searchable     bool                   `yaml:"searchable"`
	Filter         bool                   `yaml:"filter"`
	Sortable       bool                   `yaml:"sortable"`
	Agg            bool                   `yaml:"agg"`
	ElementType    string                 `yaml:"element_type"`
	Analyzer       string                 `yaml:"analyzer"`
	SearchAnalyzer string                 `yaml:"search_analyzer"`
	Format         string                 `yaml:"format"`
	Fields         map[string]FieldConfig `yaml:"fields"`
	CopyTo         []string               `yaml:"copy_to"`
}

type MappingOptionsConfig struct {
	Dynamic bool                   `yaml:"dynamic"`
	Extra   map[string]interface{} `yaml:"extra"`
}

type DictionaryConfig struct {
	Path string `yaml:"path"`
}

func LoadAppConfig(path string) (*AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read app config: %w", err)
	}

	var cfg AppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse app config: %w", err)
	}

	setDefaults(&cfg)
	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func setDefaults(cfg *AppConfig) {
	if cfg.Server.APIPort == 0 {
		cfg.Server.APIPort = 8090
	}
	if cfg.Server.GUIPort == 0 {
		cfg.Server.GUIPort = 8091
	}
	if cfg.Server.ReadTimeout == "" {
		cfg.Server.ReadTimeout = "10s"
	}
	if cfg.Security.TokenExpiry == "" {
		cfg.Security.TokenExpiry = "24h"
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}
	if cfg.Zinc.Default == "" && len(cfg.Zinc.Clusters) > 0 {
		for k := range cfg.Zinc.Clusters {
			cfg.Zinc.Default = k
			break
		}
	}
	if cfg.Sync.BatchSize == 0 {
		cfg.Sync.BatchSize = 500
	}
	if cfg.Sync.MaxParallelIndexes == 0 {
		cfg.Sync.MaxParallelIndexes = 3
	}
	if cfg.Sync.QueryTimeout == "" {
		cfg.Sync.QueryTimeout = "60s"
	}
	if cfg.Sync.IncrementalInterval == "" {
		cfg.Sync.IncrementalInterval = "5m"
	}
	if cfg.Sync.FullRebuildTime == "" {
		cfg.Sync.FullRebuildTime = "02:00"
	}
	if cfg.Storage == "" {
		cfg.Storage = "file"
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "./data"
	}
	if cfg.Synonyms == "" {
		cfg.Synonyms = "./data/dict/synonyms.txt"
	}
	if cfg.Env == "" {
		cfg.Env = "dev"
	}
}

func validate(cfg *AppConfig) error {
	if cfg.Security.JWTSecret == "" {
		return fmt.Errorf("security.jwt_secret is required")
	}
	if len(cfg.Zinc.Clusters) == 0 {
		return fmt.Errorf("at least one zinc cluster required")
	}
	return nil
}

func LoadDataSources(path string) (map[string]*DataSourceConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read datasources: %w", err)
	}

	var cfg struct {
		DataSources []*DataSourceConfig `yaml:"datasources"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse datasources: %w", err)
	}

	result := make(map[string]*DataSourceConfig)
	for _, ds := range cfg.DataSources {
		if ds.MaxOpen == 0 {
			ds.MaxOpen = 5
		}
		if ds.MaxIdle == 0 {
			ds.MaxIdle = 5
		}
		result[ds.Name] = ds
	}
	return result, nil
}

func LoadIndexConfig(dir string) (map[string]*IndexConfig, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read index dir: %w", err)
	}

	result := make(map[string]*IndexConfig)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		if strings.HasPrefix(entry.Name(), "_") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read index config %s: %w", entry.Name(), err)
		}

		var cfg IndexConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parse index config %s: %w", entry.Name(), err)
		}

		setIndexDefaults(&cfg)
		if err := validateIndexConfig(&cfg); err != nil {
			return nil, fmt.Errorf("validate index config %s: %w", entry.Name(), err)
		}

		name := strings.TrimSuffix(entry.Name(), ".yaml")
		cfg.Index.Name = name
		result[name] = &cfg
	}

	return result, nil
}

func setIndexDefaults(cfg *IndexConfig) {
	if cfg.Index.Alias == false {
		cfg.Index.Alias = true
	}
	if cfg.Index.MinWordLen == 0 {
		cfg.Index.MinWordLen = 2
	}
	if cfg.Index.Analyzer == "" {
		cfg.Index.Analyzer = "jieba_std"
	}
	if cfg.Index.SearchAnalyzer == "" {
		cfg.Index.SearchAnalyzer = "jieba_search"
	}
	for name, field := range cfg.Index.Fields {
		if field.Type == "" {
			field.Type = "text"
		}
		cfg.Index.Fields[name] = field
	}
	if cfg.Source.IncrementalField == "" {
		cfg.Source.IncrementalField = "update_time"
	}
}

func validateIndexConfig(cfg *IndexConfig) error {
	if cfg.Source.SQLQuery == "" {
		return fmt.Errorf("source.sql_query is required")
	}
	if cfg.Index.Name == "" {
		return fmt.Errorf("index.name is required")
	}
	return nil
}

func ParseDuration(s string) (time.Duration, error) {
	return time.ParseDuration(s)
}

func GetIndexConfigPath(name string) string {
	return filepath.Join("config", "indexes", name+".yaml")
}

// ConfigVersion 返回索引配置文件的内容 SHA256（前 16 位），作为配置版本标识（Q60）
func ConfigVersion(dir, name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, name+".yaml"))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:8]), nil
}

// ParseIndexConfig 解析单个索引配置内容（不落盘），用于保存前校验
func ParseIndexConfig(name string, data []byte) (*IndexConfig, error) {
	var cfg IndexConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("yaml 解析失败: %w", err)
	}
	setIndexDefaults(&cfg)
	cfg.Index.Name = name
	if err := validateIndexConfig(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// AcquireIndexCreateLock 原子创建索引配置占位文件（O_CREATE|O_EXCL）：
// 检查+创建原子化，防并发同名创建竞态（TOCTOU）。成功后由 SaveIndexConfig 覆盖占位。
func AcquireIndexCreateLock(dir, name string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, name+".yaml"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	return f.Close()
}

// SaveIndexConfig 原子写索引配置：写 .tmp → 校验 → rename 覆盖 → 保留 .bak（Q4 原子写）
func SaveIndexConfig(dir, name string, data []byte) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 保存前校验
	if _, err := ParseIndexConfig(name, data); err != nil {
		return err
	}

	path := filepath.Join(dir, name+".yaml")

	// 备份现有文件
	if _, err := os.Stat(path); err == nil {
		bak, err := os.ReadFile(path)
		if err == nil {
			os.WriteFile(path+".bak", bak, 0644)
		}
	}

	// 写临时文件再 rename（原子）
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("写临时文件失败: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename 失败: %w", err)
	}
	return nil
}

// DeleteIndexConfig 删除索引配置文件（含 .bak）
func DeleteIndexConfig(dir, name string) error {
	path := filepath.Join(dir, name+".yaml")
	os.Remove(path + ".bak")
	return os.Remove(path)
}

// IndexConfigExists 判断索引配置文件是否存在
func IndexConfigExists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name+".yaml"))
	return err == nil
}
