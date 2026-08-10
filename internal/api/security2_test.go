package api

// 第二波攻击测试：update/delete 非法名 / JWT 伪造提权 / 并发竞态 / toggle 边界

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func b64RawURLDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}

func b64RawURLEncode(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}

func osReadDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names, nil
}

// ===== 攻击 7：update/delete 非法索引名（URL 编码绕过）=====
func TestAttackUpdateDeleteIndexName(t *testing.T) {
	_, httpSrv := newSecurityServer(t)
	token := loginToken(t, httpSrv.URL, "admin", "admin123")

	body := "source:\n  sql_query: \"SELECT 1\"\nindex:\n  name: x\n"
	attacks := []string{"..", "%2e%2e", "a%20b", "..%5c..%5cetc", "%5fprivate"}
	for _, name := range attacks {
		// update
		code, _ := doJSON(t, httpSrv.URL, "PUT", "/api/v1/indexes/"+name, map[string]interface{}{"content": body}, token)
		if code == 200 {
			t.Errorf("update invalid name %q accepted (200)", name)
		} else {
			t.Logf("update %q blocked -> %d", name, code)
		}
		// delete
		code2, _ := doJSON(t, httpSrv.URL, "DELETE", "/api/v1/indexes/"+name, nil, token)
		if code2 == 200 {
			t.Errorf("delete invalid name %q accepted (200)", name)
		} else {
			t.Logf("delete %q blocked -> %d", name, code2)
		}
	}
}

// ===== 攻击 8：JWT 伪造提权（篡改 role 为 admin）=====
func TestAttackJWTForgeRole(t *testing.T) {
	_, httpSrv := newSecurityServer(t)

	// 用 viewer 的合法签名但篡改 payload role=admin（需密钥——假设攻击者拿到 viewer token 无法改 payload）
	// 真实场景：无密钥无法伪造。测试篡改后的 token 应被拒绝（签名校验）
	vt := loginToken(t, httpSrv.URL, "viewer", "viewer123")
	parts := strings.Split(vt, ".")
	if len(parts) != 3 {
		t.Fatalf("jwt format: %d parts", len(parts))
	}
	// 篡改 payload（base64url 解码→改 role→重编码），保留原签名
	payload := decodeB64URL(t, parts[1])
	var claims map[string]interface{}
	json.Unmarshal([]byte(payload), &claims)
	claims["role"] = "admin"
	claims["username"] = "admin"
	payload2, _ := json.Marshal(claims)
	forged := parts[0] + "." + encodeB64URL(string(payload2)) + "." + parts[2]

	code, body := doJSON(t, httpSrv.URL, "GET", "/api/v1/users", nil, forged)
	if code != 401 && code != 403 {
		t.Errorf("forged JWT: want 401/403, got %d (%s)", code, body)
	} else {
		t.Logf("forged JWT rejected -> %d", code)
	}
}

// ===== 攻击 9：并发创建同名索引（竞态）=====
func TestAttackConcurrentCreateSameIndex(t *testing.T) {
	srv, httpSrv := newSecurityServer(t)
	token := loginToken(t, httpSrv.URL, "admin", "admin123")
	body := "source:\n  sql_query: \"SELECT 1\"\nindex:\n  name: race\n"

	var wg sync.WaitGroup
	codes := make([]int, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c, _ := doJSON(t, httpSrv.URL, "POST", "/api/v1/indexes", map[string]interface{}{"name": "race", "content": body}, token)
			codes[i] = c
		}(i)
	}
	wg.Wait()

	ok := 0
	conflict := 0
	for _, c := range codes {
		if c == 200 {
			ok++
		}
		if c == 409 || c == 400 {
			conflict++
		}
	}
	t.Logf("concurrent create: 200 x%d, rejected x%d", ok, conflict)
	if ok > 1 {
		t.Errorf("race: %d concurrent creates succeeded (should be 1)", ok)
	}
	// 主文件应只存在一份（.bak 备份是正常产物，排除）
	if n := countFiles(srv.indexesDir, "race.yaml"); n != 1 {
		t.Errorf("race: %d race.yaml files", n)
	}
}

func countFiles(dir, prefix string) int {
	entries, err := osReadDir(dir)
	if err != nil {
		return -1
	}
	n := 0
	for _, e := range entries {
		if strings.HasPrefix(e, prefix) && !strings.HasSuffix(e, ".bak") {
			n++
		}
	}
	return n
}

// ===== 攻击 10：toggle 不存在 id / 非法 id =====
func TestAttackScheduleToggleEdge(t *testing.T) {
	srv, httpSrv := newSecurityServer(t)
	token := loginToken(t, httpSrv.URL, "admin", "admin123")
	srv.sched = newTestScheduler(t)

	for _, id := range []string{"0", "-1", "abc", "999999"} {
		code, body := doJSON(t, httpSrv.URL, "PUT", "/api/v1/schedules/"+id+"/status", map[string]interface{}{"enabled": true}, token)
		if code == 200 {
			t.Errorf("toggle nonexistent/invalid id %q accepted (200): %s", id, body)
		} else {
			t.Logf("toggle %q blocked -> %d", id, code)
		}
	}
}

func decodeB64URL(t *testing.T, s string) string {
	t.Helper()
	b, err := b64RawURLDecode(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func encodeB64URL(s string) string {
	return b64RawURLEncode(s)
}

var _ = fmt.Sprintf
var _ = time.Now
var _ = httptest.NewServer
var _ = bytes.NewReader
var _ = io.ReadAll
