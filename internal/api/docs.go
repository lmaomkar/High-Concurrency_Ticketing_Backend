package api

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

func RegisterDocs(r *gin.Engine) {
	r.GET("/docs", func(c *gin.Context) {
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, `<!doctype html>
<html><head><title>Evently Docs</title></head>
<body>
<redoc spec-url="/openapi.yaml"></redoc>
<script src="https://cdn.redoc.ly/redoc/latest/bundles/redoc.standalone.js"></script>
</body></html>`)
	})
	r.GET("/openapi.yaml", func(c *gin.Context) {
		// Try multiple possible paths for the openapi.yaml file
		possiblePaths := []string{
			"/docs/openapi.yaml",                       // Docker absolute path
			"docs/openapi.yaml",                        // Local relative path
			"./docs/openapi.yaml",                      // Local relative path with ./
			filepath.Join(".", "docs", "openapi.yaml"), // Cross-platform relative path
		}

		var content []byte
		var err error
		var usedPath string

		for _, path := range possiblePaths {
			content, err = os.ReadFile(path)
			if err == nil {
				usedPath = path
				break
			}
		}

		if err != nil {
			// Return detailed error for debugging
			c.JSON(http.StatusNotFound, gin.H{
				"error":       "openapi.yaml not found",
				"tried_paths": possiblePaths,
				"last_error":  err.Error(),
			})
			return
		}

		c.Header("Content-Type", "application/x-yaml")
		c.Header("X-Served-From", usedPath) // Debug header
		c.String(http.StatusOK, string(content))
	})
}
