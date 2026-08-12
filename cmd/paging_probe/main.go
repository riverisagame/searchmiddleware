// Zinc 分页替代方案探测：from/size、scroll、_id range
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func main() {
	client := &http.Client{}
	post := func(path string, body map[string]interface{}) (string, int) {
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "http://localhost:4081"+path, bytes.NewReader(b))
		req.SetBasicAuth("admin", "Complexpass#123")
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return err.Error(), 0
		}
		defer resp.Body.Close()
		rb, _ := io.ReadAll(resp.Body)
		return string(rb), resp.StatusCode
	}

	// 1. from/size 深分页：from=9000 size=1000
	b, c := post("/es/dev_maintenance/_search", map[string]interface{}{
		"from": 9000, "size": 1000, "query": map[string]interface{}{"match_all": map[string]interface{}{}},
		"sort": []interface{}{map[string]interface{}{"_id": map[string]interface{}{"order": "asc"}}},
	})
	var r struct {
		Hits struct {
			Hits []struct{ ID string `json:"_id"` } `json:"hits"`
		} `json:"hits"`
		Error string `json:"error"`
	}
	json.Unmarshal([]byte(b), &r)
	fmt.Printf("1. from=9000: code=%d hits=%d err=%v\n", c, len(r.Hits.Hits), r.Error)

	// 2. _id range 查询（keyset 替代）
	b2, c2 := post("/es/dev_maintenance/_search", map[string]interface{}{
		"size": 1000,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": []interface{}{
					map[string]interface{}{"match_all": map[string]interface{}{}},
					map[string]interface{}{"range": map[string]interface{}{"_id": map[string]interface{}{"gt": "1894"}}},
				},
			},
		},
	})
	var r2 struct {
		Hits struct {
			Hits []struct{ ID string `json:"_id"` } `json:"hits"`
		} `json:"hits"`
	}
	json.Unmarshal([]byte(b2), &r2)
	first := ""
	if len(r2.Hits.Hits) > 0 {
		first = r2.Hits.Hits[0].ID
	}
	fmt.Printf("2. range _id gt 1894: code=%d hits=%d first=%s\n", c2, len(r2.Hits.Hits), first)

	// 3. scroll API
	b3, c3 := post("/es/dev_maintenance/_search?scroll=1m", map[string]interface{}{
		"size": 1000, "query": map[string]interface{}{"match_all": map[string]interface{}{}},
	})
	fmt.Printf("3. scroll param: code=%d body=%s\n", c3, truncate(b3, 150))
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
