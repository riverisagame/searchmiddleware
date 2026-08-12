// Zinc 并发查询一致性探测：同一查询并发 N 次，检查结果是否稳定
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
)

func main() {
	var wg sync.WaitGroup
	var mu sync.Mutex
	bad := 0
	total := 0
	const n = 50
	results := make([]string, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			body := []byte(`{"query":{"bool":{"minimum_should_match":1,"should":[{"match":{"maintenance_name":{"query":"发动机","boost":5}}},{"match":{"sub_title":{"query":"发动机","boost":3}}}]}},"size":2,"sort":[{"_score":{"order":"desc"}}]}`)
			req, _ := http.NewRequest("POST", "http://localhost:4081/es/dev_maintenance/_search", bytes.NewReader(body))
			req.SetBasicAuth("admin", "Complexpass#123")
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				mu.Lock()
				bad++
				results[idx] = "ERR " + err.Error()
				mu.Unlock()
				return
			}
			defer resp.Body.Close()
			b, _ := io.ReadAll(resp.Body)
			var out struct {
				Hits struct {
					Total struct{ Value int `json:"value"` }
					Hits  []struct {
						ID    string  `json:"_id"`
						Score float64 `json:"_score"`
					} `json:"hits"`
				} `json:"hits"`
			}
			json.Unmarshal(b, &out)
			mu.Lock()
			total = out.Hits.Total.Value
			results[idx] = fmt.Sprintf("total=%d top1=%.4f", out.Hits.Total.Value, out.Hits.Hits[0].Score)
			if len(out.Hits.Hits) == 0 || out.Hits.Hits[0].Score < 5 {
				bad++
			}
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	fmt.Printf("concurrent=%d bad=%d total=%d\n", n, bad, total)
	for i, r := range results {
		if i < 8 {
			fmt.Printf("  [%d] %s\n", i, r)
		}
	}
}
