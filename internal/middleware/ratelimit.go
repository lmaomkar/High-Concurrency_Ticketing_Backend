package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

func RateLimit(rps int, burst int) gin.HandlerFunc {
	type bucket struct {
		tokens float64
		last   time.Time
	}
	var mu sync.Mutex
	buckets := map[string]*bucket{}
	refill := float64(rps)
	return func(c *gin.Context) {
		host, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
		if host == "" {
			host = c.ClientIP()
		}
		now := time.Now()
		mu.Lock()
		b := buckets[host]
		if b == nil {
			b = &bucket{tokens: float64(burst), last: now}
			buckets[host] = b
		}
		elapsed := now.Sub(b.last).Seconds()
		b.tokens = min(float64(burst), b.tokens+elapsed*refill)
		if b.tokens < 1 {
			mu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit"})
			return
		}
		b.tokens -= 1
		b.last = now
		mu.Unlock()
		c.Next()
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
