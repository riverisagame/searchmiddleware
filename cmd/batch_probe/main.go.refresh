// 批量写入压力探针：定位 Zinc WAL 合并崩溃的 batch 阈值
// 用法: go run ./cmd/batch_probe -batch 50 [-docs 10000]
package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

var batch = flag.Int("batch", 50, "docs per bulk batch")
var docs = flag.Int("docs", 10000, "total docs")

const zinc = "http://localhost:4081"

func main() {
	flag.Parse()
	client := &http.Client{Timeout: 60 * time.Second}
	auth := "Basic " + base64.StdEncoding.EncodeToString([]byte("admin:Complexpass#123"))

	idx := fmt.Sprintf("probe_batch_%d", time.Now().UnixNano())
	req, _ := http.NewRequest("PUT", zinc+"/es/"+idx, bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Authorization", auth)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("create index failed:", err)
		os.Exit(1)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	fmt.Printf("index=%s batch=%d docs=%d\n", idx, *batch, *docs)

	// 构造 NDJSON：每个文档 2 行（meta + doc）
	var ndjson bytes.Buffer
	for i := 0; i < *docs; i++ {
		fmt.Fprintf(&ndjson, `{"index":{"_index":%q,"_id":%q}}`+"\n", idx, fmt.Sprintf("d%06d", i))
		fmt.Fprintf(&ndjson, `{"title":"测试文档%d 发动机维修","price":%d.5}`+"\n", i, i)
	}

	sent := 0
	ok := 0
	deadline := time.Now().Add(180 * time.Second)
	for sent < *docs {
		if time.Now().After(deadline) {
			fmt.Println("TIMEOUT")
			os.Exit(3)
		}
		payload := takeDocs(&ndjson, sent, *batch)
		r, _ := http.NewRequest("POST", zinc+"/es/"+idx+"/_bulk?refresh=true", bytes.NewReader(payload))
		r.Header.Set("Authorization", auth)
		r.Header.Set("Content-Type", "application/x-ndjson")
		resp, err := client.Do(r)
		if err != nil {
			fmt.Printf("batch at %d: ERROR %v\n", sent, err)
			fmt.Println("ZINC_DEAD_OR_UNREACHABLE")
			os.Exit(2)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			fmt.Printf("batch at %d: HTTP %d %s\n", sent, resp.StatusCode, string(body[:min(120, len(body))]))
			os.Exit(4)
		}
		ok++
		sent += *batch
		if ok%25 == 0 {
			fmt.Printf("... %d/%d docs\n", sent, *docs)
		}
	}
	fmt.Printf("DONE sent=%d ok_batches=%d batch=%d\n", sent, ok, *batch)
}

func takeDocs(buf *bytes.Buffer, start, n int) []byte {
	// 从缓冲区第 start 个文档开始取 n 个文档（每个文档 2 行）
	b := buf.Bytes()
	pos := 0
	line := 0
	want := start * 2
	for pos < len(b) && line < want {
		if b[pos] == '\n' {
			line++
		}
		pos++
	}
	end := pos
	wantEnd := n * 2
	line = 0
	for end < len(b) && line < wantEnd {
		if b[end] == '\n' {
			line++
		}
		end++
	}
	return b[pos:end]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

