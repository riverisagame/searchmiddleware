package zinc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"searchmiddleware/internal/config"
)

// TestRealZinc_AnalyzeSynonym 诊断：分词结果 + 同义词扩展是否生效
func TestRealZinc_AnalyzeSynonym(t *testing.T) {
	client := NewClient(&config.ZincConfig{
		Clusters: map[string][]string{"default": {"http://localhost:4081"}},
		Default:  "default",
		Username: "admin",
		Password: "Complexpass#123",
	})
	if _, err := client.getHealthyURL("default"); err != nil {
		t.Skip("real zinc not available")
	}

	url, _ := client.getHealthyURL("")
	// 用索引的 search_analyzer 分析（jieba_search 含 synonym pipeline？）
	analyze := func(body string) map[string]interface{} {
		req, _ := http.NewRequest("POST", url+"/api/_analyze", io.NopCloser(makeReader(body)))
		req.Header.Set("Content-Type", "application/json")
		req.SetBasicAuth("admin", "Complexpass#123")
		resp, err := client.httpClient.Do(req)
		if err != nil {
			t.Fatalf("analyze: %v", err)
		}
		defer resp.Body.Close()
		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		return result
	}

	r1 := analyze(`{"text":"移动电话"}`)
	t.Logf("默认 analyze '移动电话': %v", r1)
	r2 := analyze(`{"text":"移动电话","analyzer":"jieba_search"}`)
	t.Logf("jieba_search '移动电话': %v", r2)
	r3 := analyze(`{"text":"手机"}`)
	t.Logf("默认 analyze '手机': %v", r3)

	_ = fmt.Sprint
}

type stringReader struct {
	s string
	i int
}

func makeReader(s string) *stringReader { return &stringReader{s: s} }

func (r *stringReader) Read(p []byte) (int, error) {
	if r.i >= len(r.s) {
		return 0, io.EOF
	}
	n := copy(p, r.s[r.i:])
	r.i += n
	return n, nil
}
