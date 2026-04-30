package server

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"asika/common/auth"
)

// Logger is a custom logger middleware
func Logger() gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        path := c.Request.URL.Path

        c.Next()

        latency := time.Since(start)
        statusCode := c.Writer.Status()

        slog.Info("request",
            "method", c.Request.Method,
            "path", path,
            "status", statusCode,
            "latency", latency,
            "ip", c.ClientIP(),
        )
    }
}

// AuthMiddleware creates an authentication middleware
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		skipPaths := []string{"/api/v1/auth", "/api/v1/wizard", "/login", "/wizard"}
		skip := false
		for _, p := range skipPaths {
			if strings.HasPrefix(path, p) {
				skip = true
				break
			}
		}
		if path == "/health" || skip {
			c.Next()
			return
		}

		token := extractToken(c)
		if token == "" {
			if strings.HasPrefix(path, "/api/") {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token", "code": 401})
			} else {
				c.Redirect(http.StatusFound, "/login")
			}
			c.Abort()
			return
		}

        claims, err := auth.ValidateJWT(token)
        if err != nil {
            if strings.HasPrefix(path, "/api/") {
                c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token", "code": 401})
            } else {
                c.Redirect(http.StatusFound, "/login")
            }
            c.Abort()
            return
        }

        c.Set("username", auth.GetUsername(claims))
        c.Set("role", auth.GetUserRole(claims))
        c.Set("claims", claims)

        c.Next()
    }
}

// RequireAuth requires authentication
func RequireAuth() gin.HandlerFunc {
    return func(c *gin.Context) {
        _, exists := c.Get("username")
        if !exists {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "code": 401})
            c.Abort()
            return
        }
        c.Next()
    }
}

// SSRAuthRequired redirects to login if not authenticated (for browser pages)
func SSRAuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		username, exists := c.Get("username")
		if !exists || username == nil || username.(string) == "" {
			c.Redirect(http.StatusFound, "/login")
			c.Abort()
			return
		}
		c.Next()
	}
}
func RequireRole(role string) gin.HandlerFunc {
    return func(c *gin.Context) {
        userRole, exists := c.Get("role")
        if !exists {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "code": 401})
            c.Abort()
            return
        }

        if !auth.HasPermission(userRole.(string), role) {
            c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "code": 403})
            c.Abort()
            return
        }

        c.Next()
    }
}

// RequireAnyRole requires any of the specified roles
func RequireAnyRole(roles ...string) gin.HandlerFunc {
    return func(c *gin.Context) {
        userRole, exists := c.Get("role")
        if !exists {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "code": 401})
            c.Abort()
            return
        }

        for _, r := range roles {
            if auth.HasPermission(userRole.(string), r) {
                c.Next()
                return
            }
        }

        c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "code": 403})
        c.Abort()
    }
}

// extractToken extracts the JWT token from Authorization header or cookie
func extractToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := splitToken(authHeader)
		if len(parts) == 2 {
			return parts[1]
		}
	}
	if token, err := c.Cookie("asika_token"); err == nil {
		return token
	}
	return ""
}

// splitToken splits the Authorization header into parts
func splitToken(header string) []string {
    for i, c := range header {
        if c == ' ' {
            return []string{header[:i], header[i+1:]}
        }
    }
    return nil
}
