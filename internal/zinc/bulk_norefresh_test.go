package zinc

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"searchmiddleware/internal/config"
)

// 回归（P0）：Bulk 请求不带 refresh 参数（refresh=true 触发 Zinc WAL 合并 nil panic，issue #2）
func TestBulkNoRefreshParam(t *testing.T) {
	var gotURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURI = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"message":"ok"}`))
	}))
	defer srv.Close()

	client := NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {srv.URL}},
		Default:  "default",
	})
	if err := client.Bulk("idx", []map[string]interface{}{{"_id": "1", "name": "x"}}, ""); err != nil {
		t.Fatalf("bulk: %v", err)
	}
	if strings.Contains(gotURI, "refresh") {
		t.Errorf("P0 regression: bulk URI contains refresh param: %s", gotURI)
	}
}
