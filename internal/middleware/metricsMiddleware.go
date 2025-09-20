package middleware

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/samirwankhede/lewly-pgpyewj/internal/metrics"
)

func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		metrics.HTTPRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), strconv.Itoa(c.Writer.Status())).Inc()
	}
}
