package sync

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"searchmiddleware/internal/indexer"
	"searchmiddleware/internal/logx"
	"searchmiddleware/internal/metadata"
)

type ReconcileResult struct {
	IndexName    string
	Type         string // count / id
	IndexCount   int64
	DBValidCount int64
	MissingIDs   []string
	ExtraIDs     []string
}

// ReconcileCount 一级对账：COUNT 快速对比（每次增量后可调用）
func (e *Engine) ReconcileCount(indexName string) (*metadata.ReconcileResult, error) {
	indexCfg := e.indexCfgs[indexName]
	if indexCfg == nil {
		return nil, fmt.Errorf("index config not found: %s", indexName)
	}

	readAlias := e.lifecycle.GetReadAlias(indexName)
	idxCount, err := e.countIndex(readAlias, indexCfg.Index.ZincCluster)
	if err != nil {
		return nil, fmt.Errorf("count index: %w", err)
	}
	dbCount := e.getExpectedCount(indexName)

	result := &metadata.ReconcileResult{
		IndexName:    indexName,
		Type:         "count",
		IndexCount:   idxCount,
		DBValidCount: dbCount,
	}
	e.saveReconcile(result)
	return result, nil
}

// ReconcileIDs 二级对账：id 级差集（每日全量后执行）
// 索引全部 id（scroll） vs 库有效 id（keyset）→ 缺同步（missing）/ 脏文档（extra）
func (e *Engine) ReconcileIDs(indexName string) (*metadata.ReconcileResult, error) {
	indexCfg := e.indexCfgs[indexName]
	if indexCfg == nil {
		return nil, fmt.Errorf("index config not found: %s", indexName)
	}
	builder := e.indexer[indexName]
	if builder == nil {
		return nil, fmt.Errorf("indexer not found: %s", indexName)
	}
	ds := e.dsMap[indexCfg.Source.DataSource]
	if ds == nil {
		return nil, fmt.Errorf("datasource not found: %s", indexCfg.Source.DataSource)
	}

	readAlias := e.lifecycle.GetReadAlias(indexName)

	// 1. 索引全部 id
	indexIDs, err := e.scrollAllIDs(readAlias, indexCfg.Index.ZincCluster)
	if err != nil {
		return nil, fmt.Errorf("scroll ids: %w", err)
	}

	// 2. 库有效 id（keyset 全量拉取）
	dbIDs, err := e.queryAllIDs(builder, ds)
	if err != nil {
		return nil, fmt.Errorf("query db ids: %w", err)
	}

	indexSet := toSet(indexIDs)
	dbSet := toSet(dbIDs)

	missing := diff(dbSet, indexSet) // 库有、索引缺 → 补同步
	extra := diff(indexSet, dbSet)   // 索引有、库无（脏文档）→ 删除

	result := &metadata.ReconcileResult{
		IndexName:    indexName,
		Type:         "id",
		IndexCount:   int64(len(indexIDs)),
		DBValidCount: int64(len(dbIDs)),
		MissingIDs:   jsonString(missing),
		ExtraIDs:     jsonString(extra),
	}
	e.saveReconcile(result)

	if len(missing) > 0 {
		e.createAlert(indexName, "WARN", fmt.Sprintf("reconcile: %d docs missing, %d stale", len(missing), len(extra)))
	} else if len(extra) > 0 {
		e.createAlert(indexName, "WARN", fmt.Sprintf("reconcile: %d stale docs to clean", len(extra)))
	}

	return result, nil
}

// FixReconcile 一键补同步：重建 missing、删除 extra（Q29 差异修复）
func (e *Engine) FixReconcile(indexName string, resultID uint) error {
	var rec metadata.ReconcileResult
	if err := e.metadata.First(&rec, resultID).Error; err != nil {
		return err
	}

	// 1. 重建缺失文档
	if len(rec.MissingIDs) > 0 {
		var ids []string
		if err := json.Unmarshal([]byte(rec.MissingIDs), &ids); err == nil && len(ids) > 0 {
			interfaces := make([]interface{}, len(ids))
			for i, id := range ids {
				interfaces[i] = id
			}
			if err := e.TriggerByIDs(indexName, interfaces); err != nil {
				logx.Errorf("reconcile", "fix reconcile rebuild failed: %v", err)
			}
		}
	}

	// 2. 删除脏文档
	if len(rec.ExtraIDs) > 0 {
		var ids []string
		if err := json.Unmarshal([]byte(rec.ExtraIDs), &ids); err == nil && len(ids) > 0 {
			e.deleteDocs(indexName, ids)
		}
	}

	e.metadata.Model(&metadata.ReconcileResult{}).Where("id = ?", resultID).Update("status", "fixed")
	return nil
}

func (e *Engine) deleteDocs(indexName string, ids []string) {
	indexCfg := e.indexCfgs[indexName]
	if indexCfg == nil {
		return
	}
	readAlias := e.lifecycle.GetReadAlias(indexName)
	for _, id := range ids {
		if err := e.zinc.DeleteDoc(readAlias, id, indexCfg.Index.ZincCluster); err != nil {
			logx.Errorf("reconcile", "delete doc %s/%s failed: %v", readAlias, id, err)
		}
	}
}

func (e *Engine) countIndex(index, clusterName string) (int64, error) {
	resp, err := e.zinc.Search(index, map[string]interface{}{
		"size":  0,
		"query": map[string]interface{}{"match_all": map[string]interface{}{}},
	}, clusterName)
	if err != nil {
		return 0, err
	}
	hits, ok := resp["hits"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("unexpected search response")
	}
	total, ok := hits["total"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("total missing")
	}
	return int64(total["value"].(float64)), nil
}

func (e *Engine) scrollAllIDs(index, clusterName string) ([]string, error) {
	ids := make([]string, 0)
	after := ""
	for {
		body := map[string]interface{}{
			"size":    1000,
			"_source": false,
			"query":   map[string]interface{}{"match_all": map[string]interface{}{}},
		}
		if after != "" {
			body["search_after"] = []interface{}{after}
		}
		resp, err := e.zinc.Search(index, body, clusterName)
		if err != nil {
			return nil, err
		}
		hits, ok := resp["hits"].(map[string]interface{})
		if !ok {
			break
		}
		arr, ok := hits["hits"].([]interface{})
		if !ok || len(arr) == 0 {
			break
		}
		for _, h := range arr {
			hm := h.(map[string]interface{})
			if id, ok := hm["_id"].(string); ok {
				ids = append(ids, id)
				after = id
			}
		}
	}
	return ids, nil
}

func (e *Engine) queryAllIDs(builder *indexer.DocumentBuilder, ds *sql.DB) ([]string, error) {
	base := strings.TrimSpace(builder.SourceSQL())
	pkCol := builder.PrimaryKey()
	if pkCol == "" {
		return nil, fmt.Errorf("cannot determine primary key")
	}

	// 把 SELECT 列表替换为主键列（保留 WHERE）
	upper := strings.ToUpper(base)
	fromIdx := strings.Index(upper, "FROM")
	if fromIdx == -1 {
		return nil, fmt.Errorf("no FROM in sql_query")
	}
	query := "SELECT " + pkCol + " " + base[fromIdx:]

	rows, err := ds.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query ids: %w", err)
	}
	defer rows.Close()

	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (e *Engine) saveReconcile(r *metadata.ReconcileResult) {
	e.metadata.Create(&metadata.ReconcileResult{
		IndexName:    r.IndexName,
		Type:         r.Type,
		IndexCount:   r.IndexCount,
		DBValidCount: r.DBValidCount,
		MissingIDs:   r.MissingIDs,
		ExtraIDs:     r.ExtraIDs,
		Status:       "ok",
	})
}

func jsonString(ids []string) string {
	if len(ids) == 0 {
		return "[]"
	}
	b, _ := json.Marshal(ids)
	return string(b)
}

func toSet(ids []string) map[string]bool {
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

func diff(base, target map[string]bool) []string {
	result := make([]string, 0)
	for id := range base {
		if !target[id] {
			result = append(result, id)
		}
	}
	sortStrings(result)
	return result
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

var _ = time.Now
