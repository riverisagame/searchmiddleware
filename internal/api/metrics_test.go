package api

import (
	"strings"
	"testing"
	"time"
)

// Q39 搜索观测：计数/耗时直方图/Top 关键词/5xx
func TestSearchMetricsObserve(t *testing.T) {
	m := newSearchMetrics()

	m.observe("手机", 80*time.Millisecond, false)
	m.observe("手机", 400*time.Millisecond, false)
	m.observe("发动机", 5*time.Second, true)
	m.observe("", 50*time.Millisecond, false) // 空关键词浏览

	var buf strings.Builder
	m.writePrometheus(&buf)
	out := buf.String()

	// 总数 4
	if !strings.Contains(out, "sm_search_total 4") {
		t.Errorf("total: want 4, got:\n%s", out)
	}
	// 直方图：0.05s 桶 1 个（50ms），0.1s 桶 2 个（80ms 含 50ms? 80ms>0.05 不入 0.05 桶）
	// 50ms<=0.05? 否（50>50 边界：<= 成立，50ms=0.05s 入桶）→ 0.05 桶含 50ms；80ms 不入
	if !strings.Contains(out, "sm_search_latency_seconds_bucket{le=\"0.05\"} 1") {
		t.Errorf("bucket 0.05: want 1, got:\n%s", out)
	}
	if !strings.Contains(out, "sm_search_latency_seconds_bucket{le=\"0.1\"} 2") {
		t.Errorf("bucket 0.1: want 2, got:\n%s", out)
	}
	if !strings.Contains(out, "sm_search_latency_seconds_bucket{le=\"+Inf\"} 4") {
		t.Errorf("bucket +Inf: want 4, got:\n%s", out)
	}
	if !strings.Contains(out, "sm_search_latency_seconds_count 4") {
		t.Errorf("count: want 4, got:\n%s", out)
	}
	// 5xx：1 次（发动机）
	if !strings.Contains(out, "sm_search_errors_total{type=\"5xx\"} 1") {
		t.Errorf("errors5xx: want 1, got:\n%s", out)
	}
	// Top 关键词：手机 2 次最前
	if !strings.Contains(out, "sm_search_top_keywords{keyword=\"手机\"} 2") {
		t.Errorf("top keyword 手机: want 2, got:\n%s", out)
	}
	// 空关键词不进 Top
	if strings.Contains(out, "sm_search_top_keywords{keyword=\"\"}") {
		t.Errorf("empty keyword must not appear:\n%s", out)
	}
}

// Top N 截断
func TestSearchMetricsTopN(t *testing.T) {
	m := newSearchMetrics()
	for i := 0; i < 60; i++ {
		m.observe("kw"+string(rune('0'+i/10))+string(rune('0'+i%10)), time.Millisecond, false)
	}
	top := m.topKeywords(50)
	if len(top) != 50 {
		t.Errorf("topN: want 50, got %d", len(top))
	}
}
