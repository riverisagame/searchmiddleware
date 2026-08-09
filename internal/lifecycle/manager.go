package lifecycle

import (
	"fmt"
	"strings"
	"time"

	"searchmiddleware/internal/config"
	"searchmiddleware/internal/logx"
	"searchmiddleware/internal/metadata"
	"searchmiddleware/internal/zinc"
)

type Manager struct {
	cfg       *config.AppConfig
	indexCfgs map[string]*config.IndexConfig
	metadata  *metadata.DB
	zinc      *zinc.Client
}

func NewManager(cfg *config.AppConfig, indexCfgs map[string]*config.IndexConfig, metaDB *metadata.DB, zincClient *zinc.Client) *Manager {
	return &Manager{
		cfg:       cfg,
		indexCfgs: indexCfgs,
		metadata:  metaDB,
		zinc:      zincClient,
	}
}

func (m *Manager) GetReadAlias(indexName string) string {
	prefix := m.cfg.Env + "_"
	if m.cfg.Env == "" {
		prefix = "dev_"
	}
	return prefix + indexName
}

func (m *Manager) GetWriteIndex(indexName string) string {
	var idxConfig metadata.IndexConfig
	if err := m.metadata.Where("name = ?", indexName).First(&idxConfig).Error; err != nil {
		return ""
	}
	return idxConfig.Config
}

func (m *Manager) CreateWriteIndex(indexName string) (string, error) {
	prefix := m.cfg.Env + "_"
	if m.cfg.Env == "" {
		prefix = "dev_"
	}
	writeIndex := fmt.Sprintf("%s%s_write_%d", prefix, indexName, time.Now().UnixMilli())

	mapping := m.buildMapping(indexName)
	if err := m.zinc.CreateIndex(writeIndex, mapping, m.indexCfgs[indexName].Index.ZincCluster); err != nil {
		logx.Errorf("lifecycle", "create write index failed: %v", err)
		return "", err
	}

	if err := m.metadata.Create(&metadata.IndexConfig{
		Name:    indexName,
		Config:  writeIndex,
		Version: fmt.Sprintf("%d", time.Now().UnixMilli()),
	}).Error; err != nil {
		return "", err
	}

	return writeIndex, nil
}

func (m *Manager) PrepareWriteIndex(indexName, writeIndex string, expectedDocs int) error {
	return nil
}

func (m *Manager) SwitchAlias(indexName, writeIndex string) error {
	readAlias := m.GetReadAlias(indexName)

	oldIndexes, err := m.zinc.GetAlias(readAlias, m.indexCfgs[indexName].Index.ZincCluster)
	if err != nil {
		logx.Errorf("lifecycle", "get old alias failed: %v", err)
	}

	addMap := map[string][]string{readAlias: {writeIndex}}
	removeMap := map[string][]string{}

	// GetAlias 返回 {索引名: {"aliases": {别名: {...}}}}；仅移除旧索引上的 readAlias
	for idxName := range oldIndexes {
		if idxName == writeIndex {
			continue
		}
		removeMap[readAlias] = append(removeMap[readAlias], idxName)
	}

	if err := m.zinc.AliasSwap(addMap, removeMap, m.indexCfgs[indexName].Index.ZincCluster); err != nil {
		return fmt.Errorf("alias swap failed: %w", err)
	}

	go m.cleanupOldIndexes(indexName, readAlias, writeIndex)

	return nil
}

func (m *Manager) cleanupOldIndexes(indexName, readAlias, newWriteIndex string) {
	time.Sleep(24 * time.Hour)

	oldIndexes, err := m.zinc.GetAlias(readAlias, m.indexCfgs[indexName].Index.ZincCluster)
	if err != nil {
		return
	}

	// GetAlias 返回 {索引名: {"aliases": {...}}}；删除仍指向 readAlias 的旧 write 索引（保留新索引）
	var toDelete []string
	for idxName := range oldIndexes {
		if strings.HasPrefix(idxName, m.cfg.Env+"_"+indexName+"_write_") && idxName != newWriteIndex {
			toDelete = append(toDelete, idxName)
		}
	}

	for i, idx := range toDelete {
		if i >= 2 {
			break
		}
		m.zinc.DeleteIndex(idx, m.indexCfgs[indexName].Index.ZincCluster)
	}
}

func (m *Manager) MarkWriteIndexFailed(indexName, writeIndex string) {
	m.metadata.Exec("UPDATE index_configs SET config = ? WHERE name = ?", "failed_"+writeIndex, indexName)
}

func (m *Manager) buildMapping(indexName string) map[string]interface{} {
	indexCfg := m.indexCfgs[indexName]
	if indexCfg == nil {
		return map[string]interface{}{}
	}

	properties := make(map[string]interface{})
	for name, fc := range indexCfg.Index.Fields {
		prop := map[string]interface{}{
			"type": fc.Type,
		}
		if fc.Analyzer != "" {
			prop["analyzer"] = fc.Analyzer
		}
		if fc.SearchAnalyzer != "" {
			prop["search_analyzer"] = fc.SearchAnalyzer
		}
		if fc.Format != "" {
			prop["format"] = fc.Format
		}
		if fc.ElementType != "" {
			prop["element_type"] = fc.ElementType
		}
		if fc.Sortable {
			prop["sortable"] = true
		}
		if fc.Agg {
			prop["aggregatable"] = true
		}
		if fc.Searchable {
			prop["searchable"] = true
		}
		if len(fc.CopyTo) > 0 {
			prop["copy_to"] = fc.CopyTo
		}
		if len(fc.Fields) > 0 {
			fields := make(map[string]interface{})
			for subName, subFc := range fc.Fields {
				subProp := map[string]interface{}{
					"type": subFc.Type,
				}
				if subFc.Analyzer != "" {
					subProp["analyzer"] = subFc.Analyzer
				}
				if subFc.SearchAnalyzer != "" {
					subProp["search_analyzer"] = subFc.SearchAnalyzer
				}
				fields[subName] = subProp
			}
			prop["fields"] = fields
		}
		properties[name] = prop
	}

	mapping := map[string]interface{}{
		"properties": properties,
	}

	if indexCfg.Index.MappingOptions.Dynamic {
		mapping["dynamic"] = true
	}
	for k, v := range indexCfg.Index.MappingOptions.Extra {
		mapping[k] = v
	}

	return mapping
}
