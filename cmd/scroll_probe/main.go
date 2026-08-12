// Zinc scroll API 探测
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
	auth := "Basic " + b64("admin:Complexpass#123")
	post := func(path string, body string) (string, int) {
		req, _ := http.NewRequest("POST", "http://localhost:4081"+path, bytes.NewReader([]byte(body)))
		req.Header.Set("Authorization", auth)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return err.Error(), 0
		}
		defer resp.Body.Close()
		rb, _ := io.ReadAll(resp.Body)
		return string(rb), resp.StatusCode
	}

	b1, c1 := post("/es/dev_maintenance/_search?scroll=1m", `{"size":1000,"query":{"match_all":{}},"sort":[{"_id":{"order":"asc"}}]}`)
	var r1 struct {
		ScrollID string `json:"_scroll_id"`
		Hits     struct {
			Hits []struct{ ID string `json:"_id"` } `json:"hits"`
		} `json:"hits"`
		Error string `json:"error"`
	}
	json.Unmarshal([]byte(b1), &r1)
	fmt.Printf("scroll init: code=%d error=%v scroll_id=%q hits=%d\n", c1, r1.Error, r1.ScrollID, len(r1.Hits.Hits))
	if r1.ScrollID == "" {
		fmt.Println("SCROLL NOT SUPPORTED")
		return
	}

	total := len(r1.Hits.Hits)
	last := ""
	if len(r1.Hits.Hits) > 0 {
		last = r1.Hits.Hits[len(r1.Hits.Hits)-1].ID
	}
	for i := 0; i < 3; i++ {
		b2, c2 := post("/_search/scroll", fmt.Sprintf(`{"scroll":"1m","scroll_id":%q}`, r1.ScrollID))
		var r2 struct {
			Hits struct {
				Hits []struct{ ID string `json:"_id"` } `json:"hits"`
			} `json:"hits"`
			Error string `json:"error"`
		}
		json.Unmarshal([]byte(b2), &r2)
		n := len(r2.Hits.Hits)
		f, l := "", ""
		if n > 0 {
			f = r2.Hits.Hits[0].ID
			l = r2.Hits.Hits[n-1].ID
		}
		fmt.Printf("scroll page %d: code=%d hits=%d first=%s last=%s err=%v\n", i+1, c2, n, f, l, r2.Error)
		total += n
		if n == 0 {
			break
		}
	}
	fmt.Printf("SCROLL total collected: %d\n", total)
	_ = last
}

func b64(s string) string {
	return base64([]byte(s))
}

func base64(b []byte) string {
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	out := ""
	for i := 0; i < len(b); i += 3 {
		var v int
		n := 3
		if i+3 > len(b) {
			n = len(b) - i
		}
		for j := 0; j < n; j++ {
			v = v<<8 | int(b[i+j])
		}
		v <<= (3 - n) * 8
		out += string(chars[(v>>18)&63])
		out += string(chars[(v>>12)&63])
		if n > 1 {
			out += string(chars[(v>>6)&63])
		} else {
			out += "="
		}
		if n > 2 {
			out += string(chars[v&63])
		} else {
			out += "="
		}
	}
	return out
}
