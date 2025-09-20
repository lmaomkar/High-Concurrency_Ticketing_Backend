package middleware

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// RequestLogger is a simple zap logger middleware.
func RequestLogger(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		log.Info("request",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
		)
	}
}
