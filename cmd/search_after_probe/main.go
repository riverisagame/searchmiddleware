// Zinc search_after 分页探测：验证稳定排序 + search_after 是否生效
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
	search := func(body map[string]interface{}) ([]string, error) {
		b, _ := json.Marshal(body)
		req, _ := http.NewRequest("POST", "http://localhost:4081/es/dev_maintenance/_search", bytes.NewReader(b))
		req.SetBasicAuth("admin", "Complexpass#123")
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		rb, _ := io.ReadAll(resp.Body)
		var out struct {
			Hits struct {
				Hits []struct {
					ID string `json:"_id"`
				} `json:"hits"`
			} `json:"hits"`
		}
		if err := json.Unmarshal(rb, &out); err != nil {
			return nil, err
		}
		ids := make([]string, 0, len(out.Hits.Hits))
		for _, h := range out.Hits.Hits {
			ids = append(ids, h.ID)
		}
		return ids, nil
	}

	// 分页 5 页，观察每页首尾 id 是否前进
	after := ""
	for page := 0; page < 5; page++ {
		body := map[string]interface{}{
			"size":    1000,
			"_source": false,
			"sort":    []interface{}{map[string]interface{}{"_id": map[string]interface{}{"order": "asc"}}},
			"query":   map[string]interface{}{"match_all": map[string]interface{}{}},
		}
		if after != "" {
			body["search_after"] = []interface{}{after}
		}
		ids, err := search(body)
		if err != nil {
			fmt.Printf("page %d: error %v\n", page, err)
			return
		}
		if len(ids) == 0 {
			fmt.Printf("page %d: EMPTY\n", page)
			return
		}
		fmt.Printf("page %d: count=%d first=%s last=%s\n", page, len(ids), ids[0], ids[len(ids)-1])
		after = ids[len(ids)-1]
	}
}
