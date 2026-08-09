package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"searchmiddleware/internal/api"
	"searchmiddleware/internal/auth"
	"searchmiddleware/internal/config"
	"searchmiddleware/internal/lifecycle"
	"searchmiddleware/internal/logx"
	"searchmiddleware/internal/metadata"
	"searchmiddleware/internal/scheduler"
	"searchmiddleware/internal/sync"
	"searchmiddleware/internal/web"
	"searchmiddleware/internal/zinc"
)

func main() {
	configPath := flag.String("config", "config/app.yaml", "path to app config")
	dataSourcesPath := flag.String("datasources", "config/datasources.yaml", "path to datasources config")
	indexesDir := flag.String("indexes", "config/indexes", "path to indexes config dir")
	flag.Parse()

	if len(os.Args) > 1 && os.Args[1] == "user:create" {
		createUserCmd(os.Args[2:])
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "config:check" {
		configCheckCmd(os.Args[2:])
		return
	}

	appCfg, err := config.LoadAppConfig(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Q39：日志级别初始化（debug/info/warn/error，默认 info）
	if lvl, ok := logx.ParseLevel(appCfg.LogLevel); ok {
		logx.SetLevel(lvl)
		logx.Infof("main", "log level set to %s", appCfg.LogLevel)
	}

	dsCfgs, err := config.LoadDataSources(*dataSourcesPath)
	if err != nil {
		log.Fatalf("load datasources: %v", err)
	}

	indexCfgs, err := config.LoadIndexConfig(*indexesDir)
	if err != nil {
		log.Fatalf("load indexes: %v", err)
	}

	metaDB, err := metadata.NewDB("data/metadata.db")
	if err != nil {
		log.Fatalf("open metadata db: %v", err)
	}
	if err := metaDB.AutoMigrate(); err != nil {
		log.Fatalf("migrate metadata: %v", err)
	}

	zincClient := zinc.NewClient(&appCfg.Zinc)

	dsMap := openDataSources(dsCfgs)
	defer closeDataSources(dsMap)

	lifecycleMgr := lifecycle.NewManager(appCfg, indexCfgs, metaDB, zincClient)

	syncEngine := sync.NewEngine(appCfg, indexCfgs, metaDB, zincClient, lifecycleMgr, dsMap)

	ttl, _ := time.ParseDuration(appCfg.Security.TokenExpiry)
	authMgr := auth.NewManager(appCfg.Security.JWTSecret, ttl)

	schedulerMgr := scheduler.New(metaDB, syncEngine, indexNames(indexCfgs))
	schedulerMgr.Start()

	apiServer := api.NewServer(appCfg, metaDB, zincClient, syncEngine, authMgr, indexCfgs, dsMap, schedulerMgr)

	// Q15 热加载：监听 config/indexes/*.yaml 变更 → 校验 → 回灌 DB → 更新内存
	if watcher, err := config.NewWatcher(*indexesDir, func(name string) {
		if err := apiServer.ReloadIndexConfigs(); err != nil {
			log.Printf("[config-watch] reload %s failed: %v", name, err)
			return
		}
		log.Printf("[config-watch] %s reloaded，索引需重建", name)
	}); err == nil {
		watcher.Start()
		defer watcher.Stop()
	} else {
		log.Printf("[config-watch] init failed: %v", err)
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", appCfg.Server.APIPort),
		Handler: apiServer.Router(),
	}

	// GUI port：静态资源 + /api 反向代理到 API 端口（Q13 端口分离，代理桥接）
	go startGUIServer(appCfg.Server.GUIPort, appCfg.Server.APIPort)

	go func() {
		logx.Infof("main", "API server listening on :%d", appCfg.Server.APIPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("api server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")
	schedulerMgr.Stop()
	syncEngine.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	log.Println("bye")
}

func openDataSources(cfgs map[string]*config.DataSourceConfig) map[string]*sql.DB {
	result := make(map[string]*sql.DB)
	for name, cfg := range cfgs {
		db, err := sql.Open("mysql", cfg.DSN)
		if err != nil {
			log.Printf("open datasource %s failed: %v", name, err)
			continue
		}
		db.SetMaxOpenConns(cfg.MaxOpen)
		db.SetMaxIdleConns(cfg.MaxIdle)
		result[name] = db
	}
	return result
}

func closeDataSources(dsMap map[string]*sql.DB) {
	for _, ds := range dsMap {
		ds.Close()
	}
}

func indexNames(cfgs map[string]*config.IndexConfig) map[string]bool {
	result := make(map[string]bool)
	for name := range cfgs {
		result[name] = true
	}
	return result
}

func startGUIServer(port, apiPort int) {
	handler, err := web.Handler()
	if err != nil {
		log.Printf("gui embed unavailable: %v", err)
		return
	}

	// /api 反向代理到 API 端口（GUI 单端口访问，浏览器无需跨域）
	proxy := httputil.NewSingleHostReverseProxy(&url.URL{Scheme: "http", Host: fmt.Sprintf("127.0.0.1:%d", apiPort)})

	mux := http.NewServeMux()
	mux.Handle("/api/", proxy)
	mux.Handle("/health", proxy)
	mux.Handle("/", handler)
	log.Printf("GUI server listening on :%d (api proxy -> :%d)", port, apiPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), mux))
}

// createUserCmd 引导创建用户: searchmiddleware user:create <username> <password> [admin|viewer]
func createUserCmd(args []string) {
	if len(args) < 2 {
		log.Fatal("usage: searchmiddleware user:create <username> <password> [role]")
	}
	username, password := args[0], args[1]
	role := "viewer"
	if len(args) >= 3 {
		role = args[2]
	}
	if role != "admin" && role != "viewer" {
		log.Fatalf("role must be admin or viewer, got %s", role)
	}

	metaDB, err := metadata.NewDB("data/metadata.db")
	if err != nil {
		log.Fatalf("open metadata db: %v", err)
	}
	if err := metaDB.AutoMigrate(); err != nil {
		log.Fatalf("migrate metadata: %v", err)
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}
	user := metadata.User{Username: username, Password: hash, Role: role}
	if err := metaDB.Create(&user).Error; err != nil {
		log.Fatalf("create user: %v", err)
	}
	log.Printf("user %s created with role %s", username, role)
}

// configCheckCmd 预检：全量配置校验 + SQL 试跑 + Zinc 连通性（Q4 config:check）
func configCheckCmd(args []string) {
	configPath := "config/app.yaml"
	dataSourcesPath := "config/datasources.yaml"
	indexesDir := "config/indexes"

	for i := 0; i < len(args)-1; i++ {
		switch args[i] {
		case "--config":
			configPath = args[i+1]
			i++
		case "--datasources":
			dataSourcesPath = args[i+1]
			i++
		case "--indexes":
			indexesDir = args[i+1]
			i++
		}
	}

	appCfg, err := config.LoadAppConfig(configPath)
	if err != nil {
		log.Fatalf("[FAIL] app config: %v", err)
	}
	fmt.Printf("[OK] app config (env=%s, api=:%d)\n", appCfg.Env, appCfg.Server.APIPort)

	dsCfgs, err := config.LoadDataSources(dataSourcesPath)
	if err != nil {
		log.Fatalf("[FAIL] datasources: %v", err)
	}
	for name, ds := range dsCfgs {
		db, err := sql.Open("mysql", ds.DSN)
		if err != nil {
			log.Printf("[FAIL] datasource %s: %v", name, err)
			continue
		}
		if err := db.Ping(); err != nil {
			log.Printf("[FAIL] datasource %s unreachable: %v", name, err)
		} else {
			fmt.Printf("[OK] datasource %s (read_only=%v)\n", name, ds.ReadOnly)
		}
		db.Close()
	}

	indexCfgs, err := config.LoadIndexConfig(indexesDir)
	if err != nil {
		log.Fatalf("[FAIL] indexes: %v", err)
	}
	for name, ic := range indexCfgs {
		version, _ := config.ConfigVersion(indexesDir, name)
		fmt.Printf("[OK] index %s (version=%s, ds=%s, alias=%v)\n", name, version, ic.Source.DataSource, ic.Index.Alias)
	}

	// Zinc 连通性
	zc := zinc.NewClient(&appCfg.Zinc)
	health := zc.HealthCheck(appCfg.Zinc.Default)
	allDown := true
	for url, ok := range health {
		if ok {
			fmt.Printf("[OK] zinc %s\n", url)
			allDown = false
		}
	}
	if allDown {
		log.Println("[WARN] no zinc node reachable")
	}

	fmt.Println("config:check done")
}
