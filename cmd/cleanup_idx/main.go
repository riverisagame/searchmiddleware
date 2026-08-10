// 删除损坏索引 + 残留 probe 索引
package main

import (
	"fmt"
	"net/http"
	"time"
)

func main() {
	zinc := "http://localhost:4081"
	client := &http.Client{Timeout: 30 * time.Second}
	for _, idx := range []string{
		"dev_maintenance_write_1786152495011",
		"probe_fresh",
		"dev_b10_a_1786262607106", "dev_b10_b_1786262607106",
		"dev_b10_a_1786262641524", "dev_b10_b_1786262641524",
		"dev_alias_f_a_1786202541403", "dev_alias_test_b_1786202501388", "dev_alias_v_b_1786240651953",
	} {
		req, _ := http.NewRequest("DELETE", zinc+"/api/index/"+idx, nil)
		req.SetBasicAuth("admin", "Complexpass#123")
		start := time.Now()
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("[%s] ERR after %v: %v\n", idx, time.Since(start).Round(time.Millisecond), err)
			continue
		}
		resp.Body.Close()
		fmt.Printf("[%s] %d in %v\n", idx, resp.StatusCode, time.Since(start).Round(time.Millisecond))
	}
}
