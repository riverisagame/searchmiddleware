package api

// 攻击级安全测试：索引配置 + 定时任务（路径遍历/注入/越权/畸形输入）

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"searchmiddleware/internal/auth"
	"searchmiddleware/internal/config"
	"searchmiddleware/internal/metadata"
	"searchmiddleware/internal/sync"
	"searchmiddleware/internal/zinc"
)

// 构建真实 Server（真实 router/middleware，mock zinc）
func newSecurityServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()

	// 临时 config 目录（隔离）
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "indexes"), 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.AppConfig{
		Server:  config.ServerConfig{APIPort: 8090, GUIPort: 8091, ReadTimeout: "10s"},
		Security: config.SecurityConfig{JWTSecret: "test-secret", TokenExpiry: "24h"},
		Sync:    config.SyncConfig{BatchSize: 500, MaxParallelIndexes: 3, QueryTimeout: "60s"},
		Env:     "dev",
	}
	cfg.Zinc.Clusters = map[string][]string{"default": {"http://127.0.0.1:1"}} // 不可达 zinc

	metaDB, err := metadata.NewDB("file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	if err := metaDB.AutoMigrate(); err != nil {
		t.Fatal(err)
	}

	zClient := zinc.NewClient(&cfg.Zinc)
	lifecycleMgr := nilLifecycle()
	syncEngine := sync.NewEngine(cfg, map[string]*config.IndexConfig{}, metaDB, zClient, lifecycleMgr, map[string]*sql.DB{})
	authMgr := auth.NewManager(cfg.Security.JWTSecret, 24*3600*1e9)
	indexCfgs := map[string]*config.IndexConfig{}

	srv := NewServer(cfg, metaDB, zClient, syncEngine, authMgr, indexCfgs, nil, nil)
	srv.indexesDir = filepath.Join(tmpDir, "indexes")

	// 创建用户（admin + viewer）
	metaDB.Create(&metadata.User{Username: "admin", PasswordHash: bcryptHash("admin123"), Role: "admin"})
	metaDB.Create(&metadata.User{Username: "viewer", PasswordHash: bcryptHash("viewer123"), Role: "viewer"})

	httpSrv := httptest.NewServer(srv.Router())
	t.Cleanup(httpSrv.Close)
	return srv, httpSrv
}

func bcryptHash(pw string) string {
	h, _ := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.MinCost)
	return string(h)
}

func nilLifecycle() *lifecycle.Manager {
	return nil
}

func doJSON(t *testing.T, base, method, path string, body interface{}, token string) (int, string) {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, base+path, rdr)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func loginToken(t *testing.T, base, user, pw string) string {
	t.Helper()
	code, body := doJSON(t, base, "POST", "/api/v1/auth/login", map[string]interface{}{"username": user, "password": pw}, "")
	if code != 200 {
		t.Fatalf("login %s: %d %s", user, code, body)
	}
	var r map[string]interface{}
	json.Unmarshal([]byte(body), &r)
	return r["data"].(map[string]interface{})["token"].(string)
}

// ===== 攻击 1：路径遍历（P0 候选）=====
func TestAttackIndexPathTraversal(t *testing.T) {
	srv, httpSrv := newSecurityServer(t)
	token := loginToken(t, httpSrv.URL, "admin", "admin123")
	escaped := filepath.Join("..", "..", "escaped_"+strings.ReplaceAll(t.Name(), "/", "_"))
	target := filepath.Join(srv.indexesDir, "..", "escaped_"+strings.ReplaceAll(t.Name(), "/", "_")+".yaml")

	// 攻击：name 含 ../ 试图逃逸目录写文件
	code, body := doJSON(t, httpSrv.URL, "POST", "/api/v1/indexes", map[string]interface{}{
		"name":    escaped,
		"content": "source:\n  sql_query: \"SELECT 1\"\nindex:\n  name: evil\n",
	}, token)
	t.Logf("path traversal create: code=%d body=%s", code, body)

	// 检查是否逃逸写入
	if _, err := os.Stat(target); err == nil {
		t.Errorf("P0 VULN: path traversal wrote file outside dir: %s", target)
	} else {
		t.Log("OK: no escape write")
	}
}

// ===== 攻击 2：未认证访问 =====
func TestAttackNoAuth(t *testing.T) {
	_, httpSrv := newSecurityServer(t)
	for _, p := range []string{"/api/v1/indexes", "/api/v1/schedules", "/api/v1/synonyms", "/api/v1/users"} {
		code, _ := doJSON(t, httpSrv.URL, "GET", p, nil, "")
		if code != 401 {
			t.Errorf("no-auth %s: want 401, got %d", p, code)
		}
	}
}

// ===== 攻击 3：viewer 越权（admin 接口）=====
func TestAttackViewerEscalation(t *testing.T) {
	_, httpSrv := newSecurityServer(t)
	vt := loginToken(t, httpSrv.URL, "viewer", "viewer123")

	adminOnly := []struct {
		method, path string
		body         interface{}
	}{
		{"POST", "/api/v1/indexes", map[string]interface{}{"name": "x", "content": "source:\n  sql_query: \"SELECT 1\"\nindex:\n  name: x\n"}},
		{"POST", "/api/v1/schedules", map[string]interface{}{"index_name": "x", "type": "incremental", "cron_expr": "*/5 * * * * *"}},
		{"POST", "/api/v1/users", map[string]interface{}{"username": "hack", "password": "x", "role": "admin"}},
		{"DELETE", "/api/v1/schedules/1", nil},
	}
	for _, a := range adminOnly {
		code, body := doJSON(t, httpSrv.URL, a.method, a.path, a.body, vt)
		if code == 200 {
			t.Errorf("viewer escalation: %s %s allowed (200): %s", a.method, a.path, body)
		} else {
			t.Logf("blocked: %s %s -> %d", a.method, a.path, code)
		}
	}
}

// ===== 攻击 4：非法 YAML / 畸形 content =====
func TestAttackMalformedYAML(t *testing.T) {
	_, httpSrv := newSecurityServer(t)
	token := loginToken(t, httpSrv.URL, "admin", "admin123")

	attacks := []struct {
		name    string
		content string
	}{
		{"bad yaml", "source: [unclosed"},
		{"binary", string([]byte{0x00, 0xff, 0xfe})},
		{"empty", ""},
		{"huge", strings.Repeat("a", 10<<20)}, // 10MB
	}
	for _, a := range attacks {
		code, body := doJSON(t, httpSrv.URL, "POST", "/api/v1/indexes", map[string]interface{}{
			"name": "evil", "content": a.content,
		}, token)
		if code == 200 {
			t.Errorf("malformed %s accepted (200): %s", a.name, body)
		} else {
			t.Logf("rejected %s -> %d", a.name, code)
		}
	}
}

// ===== 攻击 5：索引名非法字符 =====
func TestAttackIndexNameValidation(t *testing.T) {
	_, httpSrv := newSecurityServer(t)
	token := loginToken(t, httpSrv.URL, "admin", "admin123")

	for _, name := range []string{"_private", "a b", "a/b", "a\\b", "a:b", "..", ".", ""} {
		code, _ := doJSON(t, httpSrv.URL, "POST", "/api/v1/indexes", map[string]interface{}{
			"name": name, "content": "source:\n  sql_query: \"SELECT 1\"\nindex:\n  name: x\n",
		}, token)
		if code == 200 {
			t.Errorf("invalid index name %q accepted (200)", name)
		} else {
			t.Logf("rejected %q -> %d", name, code)
		}
	}
}

// ===== 攻击 6：定时任务非法 cron / 不存在 id =====
func TestAttackScheduleValidation(t *testing.T) {
	srv, httpSrv := newSecurityServer(t)
	token := loginToken(t, httpSrv.URL, "admin", "admin123")
	srv.sched = newTestScheduler(t)

	// 非法 cron
	bad := []string{"not-a-cron", "*/5 * * *", "999 * * * * *", "*/0 * * * * *", strings.Repeat("1 ", 1000)}
	for _, c := range bad {
		code, body := doJSON(t, httpSrv.URL, "POST", "/api/v1/schedules", map[string]interface{}{
			"index_name": "x", "type": "incremental", "cron_expr": c,
		}, token)
		if code == 200 {
			t.Errorf("invalid cron %q accepted: %s", c, body)
		} else {
			t.Logf("rejected cron %q -> %d", c, code)
		}
	}

	// 不存在的 id
	for _, p := range []string{"/api/v1/schedules/999999/status", "/api/v1/schedules/999999"} {
		code, _ := doJSON(t, httpSrv.URL, "PUT", p, map[string]interface{}{"enabled": false}, token)
		if code != 404 && code != 500 {
			t.Logf("nonexistent schedule %s -> %d (ok if not 200)", p, code)
		}
	}
	code, _ := doJSON(t, httpSrv.URL, "DELETE", "/api/v1/schedules/999999", nil, token)
	if code == 200 {
		t.Errorf("delete nonexistent schedule: should not be 200, got %d", code)
	} else {
		t.Logf("delete nonexistent -> %d", code)
	}
}

var _ = fmt.Sprintf
