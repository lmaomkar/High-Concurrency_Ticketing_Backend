package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RedisRateLimit creates a rate limiter using Redis
func RedisRateLimit(redisClient *redis.Client, rps int, burst int) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get client IP
		clientIP := c.ClientIP()
		if clientIP == "" {
			clientIP = "unknown"
		}

		// Create rate limit key
		key := fmt.Sprintf("rate_limit:%s", clientIP)

		// Use Redis sliding window counter
		ctx := context.Background()

		// Lua script for sliding window rate limiting
		luaScript := `
			local key = KEYS[1]
			local window = tonumber(ARGV[1])
			local limit = tonumber(ARGV[2])
			local now = tonumber(ARGV[3])
			
			-- Remove old entries
			redis.call('ZREMRANGEBYSCORE', key, 0, now - window)
			
			-- Count current requests
			local current = redis.call('ZCARD', key)
			
			if current < limit then
				-- Add current request
				redis.call('ZADD', key, now, now)
				redis.call('EXPIRE', key, window)
				return {1, limit - current - 1}
			else
				return {0, 0}
			end
		`

		window := time.Duration(burst) * time.Second / time.Duration(rps)
		now := time.Now().Unix()

		result, err := redisClient.Eval(ctx, luaScript, []string{key},
			int(window.Seconds()), burst, now).Result()

		if err != nil {
			// If Redis is down, allow the request (fail open)
			c.Next()
			return
		}

		results, ok := result.([]interface{})
		if !ok || len(results) < 2 {
			c.Next() // fail open
			return
		}
		allowed := results[0].(int64)
		remaining := results[1].(int64)

		// Set rate limit headers
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", burst))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", now+int64(window.Seconds())))

		if allowed == 0 {
			c.Header("Retry-After", fmt.Sprintf("%d", int(window.Seconds())))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "Rate limit exceeded",
				"retry_after": int(window.Seconds()),
			})
			return
		}

		c.Next()
	}
}

// RedisRateLimitByUser creates a rate limiter using Redis based on user ID
func RedisRateLimitByUser(redisClient *redis.Client, rps int, burst int) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context (set by auth middleware)
		userID := c.GetString("uid")
		if userID == "" {
			// If no user ID, fall back to IP-based limiting
			RedisRateLimit(redisClient, rps, burst)(c)
			return
		}

		// Create rate limit key for user
		key := fmt.Sprintf("rate_limit_user:%s", userID)

		// Use Redis sliding window counter
		ctx := context.Background()

		// Lua script for sliding window rate limiting
		luaScript := `
			local key = KEYS[1]
			local window = tonumber(ARGV[1])
			local limit = tonumber(ARGV[2])
			local now = tonumber(ARGV[3])
			
			-- Remove old entries
			redis.call('ZREMRANGEBYSCORE', key, 0, now - window)
			
			-- Count current requests
			local current = redis.call('ZCARD', key)
			
			if current < limit then
				-- Add current request
				redis.call('ZADD', key, now, now)
				redis.call('EXPIRE', key, window)
				return {1, limit - current - 1}
			else
				return {0, 0}
			end
		`

		window := time.Duration(burst) * time.Second / time.Duration(rps)
		now := time.Now().Unix()

		result, err := redisClient.Eval(ctx, luaScript, []string{key},
			int(window.Seconds()), burst, now).Result()

		if err != nil {
			// If Redis is down, allow the request (fail open)
			c.Next()
			return
		}

		results := result.([]interface{})
		allowed := results[0].(int64)
		remaining := results[1].(int64)

		// Set rate limit headers
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", burst))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", now+int64(window.Seconds())))

		if allowed == 0 {
			c.Header("Retry-After", fmt.Sprintf("%d", int(window.Seconds())))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "Rate limit exceeded",
				"retry_after": int(window.Seconds()),
			})
			return
		}

		c.Next()
	}
}

// HybridRateLimit combines Redis and in-memory rate limiting
func HybridRateLimit(redisClient *redis.Client, rps int, burst int) gin.HandlerFunc {
	// Fallback to in-memory rate limiting if Redis is unavailable
	memoryRateLimit := RateLimit(rps, burst)

	return func(c *gin.Context) {
		// Try Redis first
		ctx := context.Background()

		// Simple Redis check
		_, err := redisClient.Ping(ctx).Result()
		if err != nil {
			// Redis is down, use in-memory fallback
			memoryRateLimit(c)
			return
		}

		// Use Redis rate limiting
		RedisRateLimit(redisClient, rps, burst)(c)
	}
}
