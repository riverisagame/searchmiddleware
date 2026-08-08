package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v3"

	"searchmiddleware/internal/auth"
	"searchmiddleware/internal/config"
	"searchmiddleware/internal/metadata"
	"searchmiddleware/internal/query"
	"searchmiddleware/internal/sync"
	"searchmiddleware/internal/zinc"
)

type Server struct {
	cfg         *config.AppConfig
	meta        *metadata.DB
	zinc        *zinc.Client
	engine      *sync.Engine
	auth        *auth.Manager
	indexCfgs   map[string]*config.IndexConfig
	indexesDir  string
	dataSources map[string]*sql.DB
	synonyms    map[string][]string
	engine2     *sync.Engine
	searchMet   *searchMetrics // Q39 搜索观测
}

func NewServer(
	cfg *config.AppConfig,
	meta *metadata.DB,
	zincClient *zinc.Client,
	engine *sync.Engine,
	authMgr *auth.Manager,
	indexCfgs map[string]*config.IndexConfig,
	dsMap map[string]*sql.DB,
) *Server {
	return &Server{
		cfg:         cfg,
		meta:        meta,
		zinc:        zincClient,
		engine:      engine,
		auth:        authMgr,
		indexCfgs:   indexCfgs,
		indexesDir:  "config/indexes",
		dataSources: dsMap,
		synonyms:    loadSynonyms(meta),
		searchMet:   newSearchMetrics(),
	}
}

func (s *Server) Router() *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	r.GET("/health", s.handleHealth)
	r.POST("/api/v1/auth/login", s.handleLogin)

	v1 := r.Group("/api/v1", s.authMiddleware(), s.rateLimitMiddleware())
	{
		v1.GET("/search", s.handleSearch)
		v1.POST("/notify", s.handleNotify)

		v1.GET("/indexes", s.adminOrViewer(s.handleListIndexes))
		v1.GET("/indexes/:name", s.adminOrViewer(s.handleGetIndex))
		v1.POST("/indexes", s.adminOnly(s.handleCreateIndex))
		v1.PUT("/indexes/:name", s.adminOnly(s.handleUpdateIndex))
		v1.DELETE("/indexes/:name", s.adminOnly(s.handleDeleteIndex))
		v1.POST("/indexes/:name/sync", s.adminOnly(s.handleSyncIndex))
		v1.POST("/indexes/:name/reconcile", s.adminOnly(s.handleReconcile))
		v1.GET("/indexes/:name/reconcile", s.adminOrViewer(s.handleListReconcile))
		v1.POST("/indexes/:name/reconcile/:id/fix", s.adminOnly(s.handleFixReconcile))

		v1.GET("/runs", s.adminOrViewer(s.handleListRuns))
		v1.GET("/logs", s.adminOrViewer(s.handleListLogs))

		v1.GET("/schedules", s.adminOrViewer(s.handleListSchedules))
		v1.POST("/schedules", s.adminOnly(s.handleCreateSchedule))
		v1.PUT("/schedules/:id", s.adminOnly(s.handleUpdateSchedule))
		v1.DELETE("/schedules/:id", s.adminOnly(s.handleDeleteSchedule))

		v1.GET("/synonyms", s.adminOrViewer(s.handleListSynonyms))
		v1.POST("/synonyms", s.adminOnly(s.handleCreateSynonym))
		v1.PUT("/synonyms/:id", s.adminOnly(s.handleUpdateSynonym))
		v1.DELETE("/synonyms/:id", s.adminOnly(s.handleDeleteSynonym))
		v1.POST("/synonyms/sync", s.adminOnly(s.handleSyncSynonymsToZinc))

		v1.GET("/metrics", s.adminOrViewer(s.handleMetrics))

		v1.GET("/alerts", s.adminOrViewer(s.handleListAlerts))

		v1.GET("/users", s.adminOnly(s.handleListUsers))
		v1.POST("/users", s.adminOnly(s.handleCreateUser))

		v1.GET("/sql/test", s.adminOnly(s.handleSQLTest))
	}

	return r
}

func loadSynonyms(meta *metadata.DB) map[string][]string {
	result := make(map[string][]string)
	var synonyms []metadata.Synonym
	meta.Find(&synonyms)
	for _, syn := range synonyms {
		var list []string
		if err := jsonUnmarshal(syn.Synonyms, &list); err == nil {
			result[syn.Word] = list
		}
	}
	return result
}

func (s *Server) handleHealth(c *gin.Context) {
	zincHealth := s.zinc.HealthCheck(s.cfg.Zinc.Default)
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"zinc":   zincHealth,
	})
}

func (s *Server) handleLogin(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "msg": "invalid body"})
		return
	}

	var user metadata.User
	if err := s.meta.Where("username = ?", req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 40101, "msg": "invalid credentials"})
		return
	}

	if !auth.CheckPassword(user.Password, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 40101, "msg": "invalid credentials"})
		return
	}

	ttl, _ := time.ParseDuration(s.cfg.Security.TokenExpiry)
	token, err := s.auth.Sign(user.ID, user.Username, user.Role, nil, user.Role == "admin")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "msg": "token issue"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"token":   token,
			"expires": ttl.String(),
			"user": gin.H{
				"id":       user.ID,
				"username": user.Username,
				"role":     user.Role,
			},
		},
	})
}

func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 40101, "msg": "missing token"})
			return
		}

		claims, err := s.auth.Verify(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 40101, "msg": "invalid token"})
			return
		}

		c.Set("claims", claims)
		c.Next()
	}
}

func (s *Server) adminOnly(h gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := c.Get("claims")
		if !ok || !s.auth.IsAdmin(claims.(*auth.Claims)) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 40301, "msg": "permission denied"})
			return
		}
		h(c)
	}
}

func (s *Server) adminOrViewer(h gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := c.Get("claims"); !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 40101, "msg": "unauthorized"})
			return
		}
		h(c)
	}
}

func (s *Server) handleSearch(c *gin.Context) {
	start := time.Now()
	keyword := c.Query("keyword")
	searchErr5xx := false
	defer func() {
		s.searchMet.observe(keyword, time.Since(start), searchErr5xx)
	}()

	indexName := c.Query("index")
	if indexName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "msg": "index required"})
		return
	}

	indexCfg, ok := s.indexCfgs[indexName]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": 40401, "msg": "index not found"})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	highlight := c.DefaultQuery("highlight", "0") == "1"

	qb := query.NewQueryBuilder(indexCfg, s.synonyms)
	req := query.SearchRequest{
		Index:     indexName,
		Keyword:   keyword,
		Page:      page,
		Limit:     limit,
		Sort:      c.Query("sort"),
		Highlight: highlight,
	}

	if siteIDStr := c.Query("site_id"); siteIDStr != "" {
		if sid, err := strconv.Atoi(siteIDStr); err == nil {
			req.SiteID = &sid
		}
	}

	if filterStr := c.Query("filter"); filterStr != "" {
		var filter map[string]interface{}
		if err := jsonUnmarshal(filterStr, &filter); err == nil {
			req.Filter = filter
		}
	}

	if aggsStr := c.Query("aggs"); aggsStr != "" {
		var aggs map[string]interface{}
		if err := jsonUnmarshal(aggsStr, &aggs); err == nil {
			req.Aggs = aggs
		}
	}

	body, err := qb.Build(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "msg": err.Error()})
		return
	}

	readAlias := s.envPrefix() + indexName
	resp, err := s.zinc.Search(readAlias, body, indexCfg.Index.ZincCluster)
	if err != nil {
		searchErr5xx = true
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 50001, "msg": "zinc unavailable"})
		return
	}

	result, err := qb.ParseResponse(resp)
	if err != nil {
		searchErr5xx = true
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "msg": err.Error()})
		return
	}

	result["page"] = page
	result["limit"] = limit

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": result})
}

func (s *Server) envPrefix() string {
	if s.cfg.Env == "" {
		return "dev_"
	}
	return s.cfg.Env + "_"
}

func (s *Server) handleNotify(c *gin.Context) {
	var req struct {
		Index string        `json:"index"`
		IDs   []interface{} `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "msg": "invalid body"})
		return
	}
	if req.Index == "" || len(req.IDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "msg": "index and ids required"})
		return
	}

	if _, ok := s.indexCfgs[req.Index]; !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": 40401, "msg": "index not found"})
		return
	}

	go s.engine.TriggerByIDs(req.Index, req.IDs)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{"accepted": true}})
}

func (s *Server) handleListIndexes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": indexNames(s.indexCfgs)})
}

func indexNames(cfgs map[string]*config.IndexConfig) []string {
	names := make([]string, 0, len(cfgs))
	for name := range cfgs {
		names = append(names, name)
	}
	return names
}

func (s *Server) handleGetIndex(c *gin.Context) {
	name := c.Param("name")
	cfg, ok := s.indexCfgs[name]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": 40401, "msg": "index not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": cfg})
}

func (s *Server) handleCreateIndex(c *gin.Context) {
	var req struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "msg": "invalid body"})
		return
	}
	if req.Name == "" || req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "msg": "name and content required"})
		return
	}
	if strings.HasPrefix(req.Name, "_") {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "msg": "index name cannot start with _"})
		return
	}
	if config.IndexConfigExists(s.indexesDir, req.Name) {
		c.JSON(http.StatusConflict, gin.H{"code": 50001, "msg": "index already exists"})
		return
	}

	if err := config.SaveIndexConfig(s.indexesDir, req.Name, []byte(req.Content)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "msg": err.Error()})
		return
	}

	if err := s.reloadIndexConfigs(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "msg": err.Error()})
		return
	}

	version, _ := config.ConfigVersion(s.indexesDir, req.Name)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "配置已保存，索引需重建", "data": gin.H{"name": req.Name, "version": version, "needs_rebuild": true}})
}

func (s *Server) handleUpdateIndex(c *gin.Context) {
	name := c.Param("name")
	if !config.IndexConfigExists(s.indexesDir, name) {
		c.JSON(http.StatusNotFound, gin.H{"code": 40401, "msg": "index not found"})
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "msg": "invalid body"})
		return
	}
	if req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "msg": "content required"})
		return
	}

	if err := config.SaveIndexConfig(s.indexesDir, name, []byte(req.Content)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "msg": err.Error()})
		return
	}

	if err := s.reloadIndexConfigs(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "msg": err.Error()})
		return
	}

	version, _ := config.ConfigVersion(s.indexesDir, name)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "配置已保存，索引需重建", "data": gin.H{"name": name, "version": version, "needs_rebuild": true}})
}

func (s *Server) handleDeleteIndex(c *gin.Context) {
	name := c.Param("name")
	if !config.IndexConfigExists(s.indexesDir, name) {
		c.JSON(http.StatusNotFound, gin.H{"code": 40401, "msg": "index not found"})
		return
	}
	if err := config.DeleteIndexConfig(s.indexesDir, name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "msg": err.Error()})
		return
	}
	if err := s.reloadIndexConfigs(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}

// reloadIndexConfigs 热加载：重读配置文件 → 回灌 DB（Q15：文件唯一真相）
// ReloadIndexConfigs 热加载：重读配置文件 → 校验 → 回灌 DB → 更新内存（供 fsnotify watcher 与 API 共用）
func (s *Server) ReloadIndexConfigs() error {
	return s.reloadIndexConfigs()
}

func (s *Server) reloadIndexConfigs() error {
	cfgs, err := config.LoadIndexConfig(s.indexesDir)
	if err != nil {
		return err
	}

	// 回灌 DB（index_configs 表）
	for name, cfg := range cfgs {
		data, _ := yaml.Marshal(cfg)
		version, _ := config.ConfigVersion(s.indexesDir, name)
		var existing metadata.IndexConfig
		if err := s.meta.Where("name = ?", name).First(&existing).Error; err == nil {
			s.meta.Model(&metadata.IndexConfig{}).Where("name = ?", name).
				Updates(map[string]interface{}{"config": string(data), "version": version})
		} else {
			s.meta.Create(&metadata.IndexConfig{Name: name, Config: string(data), Version: version})
		}
	}

	// 更新内存配置
	s.indexCfgs = cfgs
	return nil
}

func (s *Server) handleSyncIndex(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		Type string        `json:"type"`
		IDs  []interface{} `json:"ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "msg": "invalid body"})
		return
	}

	var err error
	switch req.Type {
	case "full":
		err = s.engine.TriggerFullRebuild(name)
	case "incremental":
		err = s.engine.TriggerIncremental(name)
	case "by_ids":
		err = s.engine.TriggerByIDs(name, req.IDs)
	default:
		err = s.engine.TriggerIncremental(name)
	}

	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 50001, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}

func (s *Server) handleReconcile(c *gin.Context) {
	name := c.Param("name")

	if _, ok := s.indexCfgs[name]; !ok {
		c.JSON(http.StatusNotFound, gin.H{"code": 40401, "msg": "index not found"})
		return
	}

	reconcileType := c.DefaultQuery("type", "count")

	var result *metadata.ReconcileResult
	var err error
	switch reconcileType {
	case "count":
		result, err = s.engine.ReconcileCount(name)
	case "id":
		result, err = s.engine.ReconcileIDs(name)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "msg": "type must be count or id"})
		return
	}

	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"code": 50001, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": result})
}

// handleFixReconcile 一键补同步：重建缺失 + 删除脏文档
func (s *Server) handleFixReconcile(c *gin.Context) {
	name := c.Param("name")
	idStr := c.Param("id")
	id, _ := strconv.Atoi(idStr)
	if id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "msg": "id required"})
		return
	}
	if err := s.engine.FixReconcile(name, uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}

// handleListReconcile 对账结果历史
func (s *Server) handleListReconcile(c *gin.Context) {
	var results []metadata.ReconcileResult
	q := s.meta.Order("created_at desc").Limit(50)
	if idx := c.Query("index"); idx != "" {
		q = q.Where("index_name = ?", idx)
	}
	q.Find(&results)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": results})
}

func (s *Server) handleListRuns(c *gin.Context) {
	var runs []metadata.SyncRun
	s.meta.Order("started_at desc").Limit(100).Find(&runs)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": runs})
}

func (s *Server) handleListLogs(c *gin.Context) {
	var logs []metadata.SyncLog
	q := s.meta.Order("created_at desc").Limit(100)
	if idx := c.Query("index"); idx != "" {
		q = q.Where("index_name = ?", idx)
	}
	if lvl := c.Query("level"); lvl != "" {
		q = q.Where("level = ?", lvl)
	}
	q.Find(&logs)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": logs})
}

func (s *Server) handleListSchedules(c *gin.Context) {
	var schedules []metadata.Schedule
	s.meta.Find(&schedules)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": schedules})
}

func (s *Server) handleCreateSchedule(c *gin.Context) {
	var sch metadata.Schedule
	if err := c.ShouldBindJSON(&sch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "msg": "invalid body"})
		return
	}
	if err := s.meta.Create(&sch).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": sch})
}

func (s *Server) handleUpdateSchedule(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var sch metadata.Schedule
	if err := c.ShouldBindJSON(&sch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "msg": "invalid body"})
		return
	}
	s.meta.Model(&metadata.Schedule{}).Where("id = ?", id).Updates(map[string]interface{}{
		"cron_expr": sch.CronExpr,
		"enabled":   sch.Enabled,
		"type":      sch.Type,
	})
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}

func (s *Server) handleDeleteSchedule(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	s.meta.Delete(&metadata.Schedule{}, id)
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}

// handleSyncSynonymsToZinc 同义词闭环：synonyms 表 → Zinc 格式文件（原子写）→ 触发 Zinc 重载
// REQ-002：Zinc 提供 POST /api/_reload/synonym，GUI 增删改后调用此端点即时生效
func (s *Server) handleSyncSynonymsToZinc(c *gin.Context) {
	if err := s.exportSynonymsToZinc(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{"synced": true}})
}

func (s *Server) exportSynonymsToZinc() error {
	var synonyms []metadata.Synonym
	s.meta.Find(&synonyms)

	var buf strings.Builder
	for _, syn := range synonyms {
		var list []string
		if err := jsonUnmarshal(syn.Synonyms, &list); err != nil || len(list) == 0 {
			continue
		}
		// Zinc 格式：逗号分隔双向等价（processor_synonym.go）
		line := syn.Word + "," + strings.Join(list, ",")
		buf.WriteString(line + "\n")
	}

	// 原子写（Q4 模式）：.tmp → rename
	dir := filepath.Dir(s.cfg.Synonyms)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp := s.cfg.Synonyms + ".tmp"
	if err := os.WriteFile(tmp, []byte(buf.String()), 0644); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.cfg.Synonyms); err != nil {
		os.Remove(tmp)
		return err
	}

	// 触发 Zinc 热重载（REQ-002）
	if s.zinc == nil {
		return nil
	}
	return s.zinc.ReloadSynonym(s.cfg.Zinc.Default)
}

func (s *Server) handleListSynonyms(c *gin.Context) {
	var synonyms []metadata.Synonym
	s.meta.Find(&synonyms)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": synonyms})
}

func (s *Server) handleCreateSynonym(c *gin.Context) {
	var syn metadata.Synonym
	if err := c.ShouldBindJSON(&syn); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "msg": "invalid body"})
		return
	}
	if err := s.meta.Create(&syn).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "msg": err.Error()})
		return
	}
	s.reloadSynonyms()
	if err := s.exportSynonymsToZinc(); err != nil {
		log.Printf("synonym sync to zinc failed: %v", err)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": syn})
}

func (s *Server) handleUpdateSynonym(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var syn metadata.Synonym
	if err := c.ShouldBindJSON(&syn); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "msg": "invalid body"})
		return
	}
	s.meta.Model(&metadata.Synonym{}).Where("id = ?", id).Updates(map[string]interface{}{
		"word":     syn.Word,
		"synonyms": syn.Synonyms,
		"indexes":  syn.Indexes,
	})
	s.reloadSynonyms()
	if err := s.exportSynonymsToZinc(); err != nil {
		log.Printf("synonym sync to zinc failed: %v", err)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}

func (s *Server) handleDeleteSynonym(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	s.meta.Delete(&metadata.Synonym{}, id)
	s.reloadSynonyms()
	if err := s.exportSynonymsToZinc(); err != nil {
		log.Printf("synonym sync to zinc failed: %v", err)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok"})
}

func (s *Server) reloadSynonyms() {
	s.synonyms = loadSynonyms(s.meta)
}

// handleMetrics Prometheus 文本格式指标（Q30 预留，标准库手写零依赖）
func (s *Server) handleMetrics(c *gin.Context) {
	var buf strings.Builder
	buf.WriteString("# HELP sm_search_total 搜索请求累计\n# TYPE sm_search_total counter\n")
	s.searchMet.writePrometheus(&buf)

	buf.WriteString("# HELP sm_index_docs 索引文档数（最近一次对账 COUNT）\n# TYPE sm_index_docs gauge\n")
	var reconciles []metadata.ReconcileResult
	s.meta.Where("type = ?", "count").Find(&reconciles)
	latest := make(map[string]metadata.ReconcileResult)
	for _, r := range reconciles {
		if prev, ok := latest[r.IndexName]; !ok || r.ID > prev.ID {
			latest[r.IndexName] = r
		}
	}
	for idx, r := range latest {
		buf.WriteString(fmt.Sprintf("sm_index_docs{index=\"%s\"} %d\n", idx, r.IndexCount))
	}

	buf.WriteString("# HELP sm_sync_runs_total 同步任务次数\n# TYPE sm_sync_runs_total counter\n")
	var runs []metadata.SyncRun
	s.meta.Find(&runs)
	statusCount := make(map[string]map[string]int64)
	for _, run := range runs {
		if statusCount[run.IndexName] == nil {
			statusCount[run.IndexName] = make(map[string]int64)
		}
		statusCount[run.IndexName][run.Status]++
	}
	for idx, statuses := range statusCount {
		for status, n := range statuses {
			buf.WriteString(fmt.Sprintf("sm_sync_runs_total{index=\"%s\",status=\"%s\"} %d\n", idx, status, n))
		}
	}

	c.Data(http.StatusOK, "text/plain; version=0.0.4", []byte(buf.String()))
}

func (s *Server) handleListAlerts(c *gin.Context) {
	var alerts []metadata.SyncAlert
	s.meta.Order("created_at desc").Limit(100).Find(&alerts)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": alerts})
}

func (s *Server) handleListUsers(c *gin.Context) {
	var users []metadata.User
	s.meta.Select("id, username, role, created_at").Find(&users)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": users})
}

func (s *Server) handleCreateUser(c *gin.Context) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "msg": "invalid body"})
		return
	}
	if req.Role == "" {
		req.Role = "viewer"
	}
	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 50001, "msg": err.Error()})
		return
	}
	user := metadata.User{Username: req.Username, Password: hash, Role: req.Role}
	if err := s.meta.Create(&user).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": 50001, "msg": "username exists"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"id": user.ID, "username": user.Username, "role": user.Role}})
}

func (s *Server) handleSQLTest(c *gin.Context) {
	var req struct {
		Datasource string `json:"datasource"`
		SQL        string `json:"sql"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "msg": "invalid body"})
		return
	}

	// SELECT 白名单：防注入/写操作（Q4 安全）
	sqlTrimmed := strings.TrimSpace(req.SQL)
	sqlUpper := strings.ToUpper(sqlTrimmed)
	if !strings.HasPrefix(sqlUpper, "SELECT") {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "msg": "仅允许 SELECT 语句试跑"})
		return
	}
	if strings.Contains(sqlUpper, " INTO ") || strings.Contains(sqlUpper, "FOR UPDATE") || strings.Contains(sqlUpper, "LOCK IN SHARE") {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "msg": "禁止 SELECT INTO / 锁语句"})
		return
	}

	ds := s.dataSources[req.Datasource]
	if ds == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 40401, "msg": "datasource not found"})
		return
	}

	// 强制 LIMIT 20 预览
	finalSQL := forceLimit(sqlTrimmed, 20)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := ds.QueryContext(ctx, finalSQL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 40001, "msg": "SQL 执行失败: " + err.Error()})
		return
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	preview := make([]map[string]interface{}, 0, 20)
	values := make([]interface{}, len(cols))
	ptrs := make([]interface{}, len(cols))
	for i := range cols {
		ptrs[i] = &values[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			break
		}
		row := make(map[string]interface{}, len(cols))
		for i, col := range cols {
			v := values[i]
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			row[col] = v
		}
		preview = append(preview, row)
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "msg": "ok", "data": gin.H{
		"columns": cols,
		"rows":    preview,
		"limit":   20,
	}})
}

// forceLimit 确保 SQL 带 LIMIT 20（无则追加）
func forceLimit(sql string, n int) string {
	upper := strings.ToUpper(sql)
	if strings.Contains(upper, "LIMIT") {
		return sql
	}
	return sql + fmt.Sprintf(" LIMIT %d", n)
}

func jsonUnmarshal(s string, v interface{}) error {
	return json.Unmarshal([]byte(s), v)
}
