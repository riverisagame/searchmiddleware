// 搜索 API 限流：固定窗口 + IP 维度（Q39 429 观测配套，roadmap 12）
package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// rateLimiter 固定窗口限流器（每 IP 每窗口 N 次）
type rateLimiter struct {
	mu      sync.Mutex
	qps     int
	window  time.Duration
	last    map[string]time.Time // IP → 窗口起始
	counts  map[string]int       // IP → 窗口内计数
}

func newRateLimiter(qps int) *rateLimiter {
	if qps <= 0 {
		qps = 1
	}
	return &rateLimiter{
		qps:    qps,
		window: time.Second,
		last:   make(map[string]time.Time),
		counts: make(map[string]int),
	}
}

// allow 判断 IP 是否放行；窗口过期自动重置
func (r *rateLimiter) allow(ip string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	start, ok := r.last[ip]
	if !ok || now.Sub(start) >= r.window {
		r.last[ip] = now
		r.counts[ip] = 1
		return true
	}
	r.counts[ip]++
	return r.counts[ip] <= r.qps
}

// clientIP 取客户端 IP（含 X-Forwarded-For 首项，兼容反代）
func clientIP(c *gin.Context) string {
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		for i := 0; i < len(xff) && xff[i] != ','; i++ {
			if i == len(xff)-1 || xff[i+1] == ',' {
				return xff[:i+1]
			}
		}
	}
	return c.ClientIP()
}

// rateLimitMiddleware 限流中间件：命中 429 时计数并拒绝
func (s *Server) rateLimitMiddleware() gin.HandlerFunc {
	if !s.cfg.RateLimit.Enabled || s.cfg.RateLimit.QPS <= 0 {
		return func(c *gin.Context) { c.Next() }
	}
	rl := newRateLimiter(s.cfg.RateLimit.QPS)
	return func(c *gin.Context) {
		if !rl.allow(clientIP(c)) {
			s.searchMet.observe429()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"code": 42901, "msg": "too many requests"})
			return
		}
		c.Next()
	}
}
