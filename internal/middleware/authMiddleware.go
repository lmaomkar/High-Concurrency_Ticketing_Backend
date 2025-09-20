package middleware

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/samirwankhede/lewly-pgpyewj/internal/store"
)

type Claims struct {
	UserID string `json:"uid"`
	Admin  bool   `json:"adm"`
	jwt.RegisteredClaims
}

func Middleware(secret string, requireAdmin bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		tokenStr := strings.TrimPrefix(h, "Bearer ")
		token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		claims := token.Claims.(*Claims)

		// If admin is required, check both JWT claim and database
		if requireAdmin {
			if !claims.Admin {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin required"})
				return
			}

			// Double-check admin status in database
			if !isUserAdminInDB(c.Request.Context(), claims.UserID) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin privileges revoked"})
				return
			}
		}

		c.Set("uid", claims.UserID)
		c.Set("adm", claims.Admin)
		c.Next()
	}
}

func isUserAdminInDB(ctx context.Context, userID string) bool {
	// Get database connection from environment
	dbURL := getenv("POSTGRES_URL", "postgres://evently:evently@localhost:5432/evently?sslmode=disable")
	db, err := store.NewDB(ctx, dbURL, 5) // Use small connection pool for middleware
	if err != nil {
		return false
	}
	defer db.Close()

	var role string
	err = db.Pool.QueryRow(ctx, "SELECT role FROM users WHERE id = $1", userID).Scan(&role)
	if err != nil {
		return false
	}

	return role == "admin"
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// UserMiddleware is a simpler middleware that just requires authentication (not admin)
func UserMiddleware(secret string) gin.HandlerFunc {
	return Middleware(secret, false)
}

// AdminMiddleware requires admin privileges
func AdminMiddleware(secret string) gin.HandlerFunc {
	return Middleware(secret, true)
}

func Issue(secret, userID string, admin bool, ttl time.Duration) (string, error) {
	claims := &Claims{UserID: userID, Admin: admin, RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl))}}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
