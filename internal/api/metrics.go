// Q39 搜索观测指标：请求计数 / 耗时直方图 / Top 关键词 / 5xx 错误计数
// Prometheus 文本格式输出（与既有 handleMetrics 风格一致，标准库零依赖）
package api

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// searchMetrics 搜索观测的运行时统计（内存态，进程重启清零）
type searchMetrics struct {
	mu           sync.Mutex
	total        int64
	latencySum   float64 // 累计耗时（秒），用于 histogram _sum
	buckets      []float64
	bucketCnt    []int64
	errors5xx    int64
	errors429    int64
	keywords     map[string]int64
}

func newSearchMetrics() *searchMetrics {
	return &searchMetrics{
		buckets:   []float64{0.05, 0.1, 0.3, 1, 3, 10},
		bucketCnt: make([]int64, 7),
		keywords:  make(map[string]int64),
	}
}

// observe 记录一次搜索：关键词（可为空）、耗时、是否 5xx
func (m *searchMetrics) observe(keyword string, dur time.Duration, err5xx bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.total++
	if err5xx {
		m.errors5xx++
	}
	if keyword != "" {
		m.keywords[keyword]++
	}
	sec := dur.Seconds()
	m.latencySum += sec
	for i, b := range m.buckets {
		if sec <= b {
			m.bucketCnt[i]++
		}
	}
	m.bucketCnt[len(m.buckets)]++
}

// observe429 记录一次限流拒绝
func (m *searchMetrics) observe429() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errors429++
}

// topKeywords 返回 Top N 关键词（按次数降序）
func (m *searchMetrics) topKeywords(n int) []struct {
	Keyword string
	Count   int64
} {
	m.mu.Lock()
	defer m.mu.Unlock()
	type kv struct {
		k string
		c int64
	}
	all := make([]kv, 0, len(m.keywords))
	for k, c := range m.keywords {
		all = append(all, kv{k, c})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].c > all[j].c })
	if len(all) > n {
		all = all[:n]
	}
	out := make([]struct {
		Keyword string
		Count   int64
	}, len(all))
	for i, v := range all {
		out[i] = struct {
			Keyword string
			Count   int64
		}{v.k, v.c}
	}
	return out
}

// writePrometheus 输出 Prometheus 文本格式指标
func (m *searchMetrics) writePrometheus(buf *strings.Builder) {
	m.mu.Lock()
	defer m.mu.Unlock()

	buf.WriteString("# HELP sm_search_total 搜索请求累计\n# TYPE sm_search_total counter\n")
	fmt.Fprintf(buf, "sm_search_total %d\n", m.total)

	buf.WriteString("# HELP sm_search_latency_seconds 搜索耗时分布\n# TYPE sm_search_latency_seconds histogram\n")
	for i, b := range m.buckets {
		fmt.Fprintf(buf, "sm_search_latency_seconds_bucket{le=\"%g\"} %d\n", b, m.bucketCnt[i])
	}
	fmt.Fprintf(buf, "sm_search_latency_seconds_bucket{le=\"+Inf\"} %d\n", m.bucketCnt[len(m.buckets)])
	fmt.Fprintf(buf, "sm_search_latency_seconds_sum %.6f\n", m.latencySum)
	fmt.Fprintf(buf, "sm_search_latency_seconds_count %d\n", m.total)

	buf.WriteString("# HELP sm_search_errors_total 搜索错误次数\n# TYPE sm_search_errors_total counter\n")
	fmt.Fprintf(buf, "sm_search_errors_total{type=\"5xx\"} %d\n", m.errors5xx)
	fmt.Fprintf(buf, "sm_search_errors_total{type=\"429\"} %d\n", m.errors429)

	buf.WriteString("# HELP sm_search_top_keywords 搜索关键词次数\n# TYPE sm_search_top_keywords counter\n")
	type kv struct {
		k string
		c int64
	}
	all := make([]kv, 0, len(m.keywords))
	for k, c := range m.keywords {
		all = append(all, kv{k, c})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].c > all[j].c })
	if len(all) > 50 {
		all = all[:50]
	}
	for _, v := range all {
		fmt.Fprintf(buf, "sm_search_top_keywords{keyword=\"%s\"} %d\n", v.k, v.c)
	}
}
