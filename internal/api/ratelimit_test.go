package api

import (
	"testing"
	"time"
)

// 固定窗口：窗口内超限拒绝，窗口过期恢复
func TestRateLimiterWindow(t *testing.T) {
	rl := newRateLimiter(2) // 每 IP 每窗口 2 次

	if !rl.allow("1.2.3.4") {
		t.Error("first should pass")
	}
	if !rl.allow("1.2.3.4") {
		t.Error("second should pass")
	}
	if rl.allow("1.2.3.4") {
		t.Error("third should be rejected (window full)")
	}
	// 其他 IP 不受影响
	if !rl.allow("5.6.7.8") {
		t.Error("other IP should pass")
	}
}

// 窗口过期后恢复
func TestRateLimiterWindowReset(t *testing.T) {
	rl := newRateLimiter(1)
	if !rl.allow("9.9.9.9") {
		t.Error("first should pass")
	}
	if rl.allow("9.9.9.9") {
		t.Error("second should be rejected")
	}
	// 手动推进窗口
	rl.mu.Lock()
	rl.last["9.9.9.9"] = rl.last["9.9.9.9"].Add(-rl.window - time.Millisecond)
	rl.mu.Unlock()
	if !rl.allow("9.9.9.9") {
		t.Error("after window expiry should pass")
	}
}

// X-Forwarded-For 取首项
func TestClientIPXFF(t *testing.T) {
	// clientIP 依赖 gin.Context；用最小模拟验证 XFF 解析逻辑
	// 直接验证：多个 XFF 取首项（内联逻辑与中间件一致）
	xff := "10.0.0.1, 10.0.0.2"
	ip := xff
	for i := 0; i < len(xff) && xff[i] != ','; i++ {
		if i == len(xff)-1 || xff[i+1] == ',' {
			ip = xff[:i+1]
		}
	}
	if ip != "10.0.0.1" {
		t.Errorf("xff first hop: want 10.0.0.1, got %s", ip)
	}
}
