package sync

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"searchmiddleware/internal/config"
	"searchmiddleware/internal/indexer"
	"searchmiddleware/internal/lifecycle"
	"searchmiddleware/internal/metadata"
	"searchmiddleware/internal/zinc"
)

type Engine struct {
	cfg       *config.AppConfig
	indexCfgs map[string]*config.IndexConfig
	metadata  *metadata.DB
	zinc      *zinc.Client
	lifecycle *lifecycle.Manager
	indexer   map[string]*indexer.DocumentBuilder
	dsMap     map[string]*sql.DB

	mu      sync.Mutex
	running map[string]bool
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

type sqlDBWrapper struct {
	db *sql.DB
}

func NewEngine(
	cfg *config.AppConfig,
	indexCfgs map[string]*config.IndexConfig,
	metaDB *metadata.DB,
	zincClient *zinc.Client,
	lifecycleMgr *lifecycle.Manager,
	dsMap map[string]*sql.DB,
) *Engine {
	indexerMap := make(map[string]*indexer.DocumentBuilder)
	for name, idxCfg := range indexCfgs {
		ds := dsMap[idxCfg.Source.DataSource]
		if ds != nil {
			indexerMap[name] = indexer.NewDocumentBuilder(idxCfg, ds)
		}
	}

	return &Engine{
		cfg:       cfg,
		indexCfgs: indexCfgs,
		metadata:  metaDB,
		zinc:      zincClient,
		lifecycle: lifecycleMgr,
		indexer:   indexerMap,
		dsMap:     dsMap,
		running:   make(map[string]bool),
		stopCh:    make(chan struct{}),
	}
}

func (e *Engine) Start() {
	e.wg.Add(1)
	go e.retryFailedBatchesLoop()
}

func (e *Engine) Stop() {
	close(e.stopCh)
	e.wg.Wait()
}

func (e *Engine) TriggerFullRebuild(indexName string) error {
	return e.runSync(indexName, "full", nil)
}

func (e *Engine) TriggerIncremental(indexName string) error {
	return e.runSync(indexName, "incremental", nil)
}

func (e *Engine) TriggerByIDs(indexName string, ids []interface{}) error {
	return e.runSync(indexName, "by_ids", ids)
}

func (e *Engine) runSync(indexName, syncType string, ids []interface{}) error {
	if !e.tryLock(indexName) {
		e.logSync(indexName, syncType, "skipped", 0, 0, 0, "already running")
		return fmt.Errorf("sync already running for %s", indexName)
	}
	defer e.unlock(indexName)

	indexCfg := e.indexCfgs[indexName]
	if indexCfg == nil {
		return fmt.Errorf("index config not found: %s", indexName)
	}

	builder := e.indexer[indexName]
	if builder == nil {
		return fmt.Errorf("indexer not found: %s", indexName)
	}

	_ = e.lifecycle.GetReadAlias(indexName)
	writeIndex := e.lifecycle.GetWriteIndex(indexName)
	if writeIndex == "" {
		var err error
		writeIndex, err = e.lifecycle.CreateWriteIndex(indexName)
		if err != nil {
			e.logSync(indexName, syncType, "failed", 0, 0, 0, "create write index: "+err.Error())
			e.createAlert(indexName, "ERROR", "create write index failed: "+err.Error())
			return err
		}
	}

	startTime := time.Now()
	var result *indexer.BuildResult
	var err error

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	switch syncType {
	case "full":
		result, err = builder.BuildFull(ctx)
	case "incremental":
		cursor := e.getCursor(indexName)
		result, err = builder.BuildIncremental(ctx, cursor)
	case "by_ids":
		result, err = builder.BuildByIDs(ctx, ids)
	default:
		return fmt.Errorf("unknown sync type: %s", syncType)
	}

	durationMs := time.Since(startTime).Milliseconds()

	if err != nil {
		e.logSync(indexName, syncType, "failed", 0, durationMs, 0, err.Error())
		e.createAlert(indexName, "ERROR", fmt.Sprintf("sync failed: %v", err))
		return err
	}

	throughput := float64(result.Count) / (float64(durationMs) / 1000)

	if syncType == "full" {
		if err := e.lifecycle.PrepareWriteIndex(indexName, writeIndex, result.Count); err != nil {
			e.logSync(indexName, syncType, "failed", 0, durationMs, throughput, err.Error())
			return err
		}
	}

	if result.Count > 0 {
		batchSize := e.cfg.Sync.BatchSize
		failedIDs := []string{}
		for i := 0; i < len(result.Docs); i += batchSize {
			end := i + batchSize
			if end > len(result.Docs) {
				end = len(result.Docs)
			}
			batch := result.Docs[i:end]
			if err := e.zinc.Bulk(writeIndex, batch, indexCfg.Index.ZincCluster); err != nil {
				for _, doc := range batch {
					if id, ok := doc["_id"]; ok {
						failedIDs = append(failedIDs, fmt.Sprintf("%v", id))
					}
				}
				log.Printf("bulk failed for %s batch %d-%d: %v", indexName, i, end, err)
			}
		}

		if len(failedIDs) > 0 {
			e.logSync(indexName, syncType, "partial", result.Count, durationMs, throughput, fmt.Sprintf("failed_ids: %d", len(failedIDs)))
			e.saveFailedIDs(indexName, syncType, failedIDs)
			e.createAlert(indexName, "WARN", fmt.Sprintf("bulk partial failure: %d docs", len(failedIDs)))
		} else {
			e.logSync(indexName, syncType, "success", result.Count, durationMs, throughput, "")
		}
	} else {
		e.logSync(indexName, syncType, "success", 0, durationMs, throughput, "no changes")
	}

	if syncType == "incremental" && result.LastCursor != "" {
		e.updateCursor(indexName, result.LastCursor)
	}

	if syncType == "full" {
		expectedCount := e.getExpectedCount(indexName)
		if expectedCount > 0 && float64(result.Count)/float64(expectedCount) < 0.9 {
			e.logSync(indexName, syncType, "failed", result.Count, durationMs, throughput, fmt.Sprintf("90%% gate failed: got %d, expected %d", result.Count, expectedCount))
			e.lifecycle.MarkWriteIndexFailed(indexName, writeIndex)
			return fmt.Errorf("90%% gate failed: %d/%d", result.Count, expectedCount)
		}

		if err := e.lifecycle.SwitchAlias(indexName, writeIndex); err != nil {
			e.logSync(indexName, syncType, "failed", result.Count, durationMs, throughput, err.Error())
			return err
		}

		// Q29：每日全量兜底 = 对账 + 软删清理合并执行（异步，避免延长切换响应）
		go func(idx string) {
			if _, err := e.ReconcileIDs(idx); err != nil {
				log.Printf("post-full reconcile %s failed: %v", idx, err)
			}
		}(indexName)
	}

	return nil
}

func (e *Engine) getCursor(indexName string) string {
	var cursor metadata.SyncCursor
	if err := e.metadata.Where("index_name = ?", indexName).First(&cursor).Error; err != nil {
		return ""
	}
	return cursor.CursorValue
}

func (e *Engine) updateCursor(indexName, cursor string) {
	e.metadata.Exec("INSERT OR REPLACE INTO sync_cursors (index_name, cursor_value, updated_at) VALUES (?, ?, ?)",
		indexName, cursor, time.Now())
}

func (e *Engine) getExpectedCount(indexName string) int64 {
	indexCfg := e.indexCfgs[indexName]
	if indexCfg == nil {
		return 0
	}

	countQuery := strings.Replace(indexCfg.Source.SQLQuery,
		strings.Split(strings.ToUpper(indexCfg.Source.SQLQuery), "FROM")[0],
		"SELECT COUNT(*)", 1)

	ds := e.dsMap[indexCfg.Source.DataSource]
	if ds == nil {
		return 0
	}

	var count int64
	ds.QueryRow(countQuery).Scan(&count)
	return count
}

func (e *Engine) logSync(indexName, syncType, status string, rows int, durationMs int64, throughput float64, msg string) {
	run := metadata.SyncRun{
		IndexName:  indexName,
		Type:       syncType,
		Trigger:    "manual",
		Status:     status,
		RowsCount:  int64(rows),
		DurationMs: durationMs,
		Throughput: throughput,
		StartedAt:  time.Now().Add(-time.Duration(durationMs) * time.Millisecond),
	}
	e.metadata.Create(&run)

	if msg != "" {
		e.metadata.Create(&metadata.SyncLog{
			RunID:      run.ID,
			IndexName:  indexName,
			Level:      "INFO",
			Message:    msg,
			Task:       syncType,
			DurationMs: durationMs,
		})
	}
}

func (e *Engine) saveFailedIDs(indexName, syncType string, ids []string) {
	data, _ := json.Marshal(ids)
	e.metadata.Exec("INSERT INTO sync_logs (run_id, index_name, level, message, task, duration_ms) VALUES (?, ?, 'WARN', ?, ?, 0)",
		0, indexName, string(data), syncType)
}

func (e *Engine) createAlert(indexName, level, msg string) {
	alert := metadata.SyncAlert{
		IndexName: indexName,
		Level:     level,
		Message:   msg,
	}
	e.metadata.Create(&alert)
}

func (e *Engine) tryLock(indexName string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running[indexName] {
		return false
	}
	e.running[indexName] = true
	return true
}

func (e *Engine) unlock(indexName string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.running[indexName] = false
}

func (e *Engine) retryFailedBatchesLoop() {
	defer e.wg.Done()
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.retryFailedBatches()
		}
	}
}

func (e *Engine) retryFailedBatches() {
	var logs []metadata.SyncLog
	e.metadata.Where("level = 'WARN' AND message LIKE '%failed_ids%' AND created_at > ?", time.Now().Add(-24*time.Hour)).Find(&logs)

	for _, entry := range logs {
		var ids []string
		json.Unmarshal([]byte(entry.Message), &ids)
		if len(ids) == 0 {
			continue
		}

		var idInterfaces []interface{}
		for _, id := range ids {
			idInterfaces = append(idInterfaces, id)
		}

		if err := e.TriggerByIDs(entry.IndexName, idInterfaces); err != nil {
			log.Printf("retry failed for %s: %v", entry.IndexName, err)
		}
	}
}

func (e *Engine) GetRunningStatus() map[string]bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	result := make(map[string]bool)
	for k, v := range e.running {
		result[k] = v
	}
	return result
}
