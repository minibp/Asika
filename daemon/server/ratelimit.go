package server

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

var visitors sync.Map

func getVisitor(ip string, r rate.Limit, b int) *rate.Limiter {
	v, exists := visitors.Load(ip)
	if !exists {
		limiter := rate.NewLimiter(r, b)
		visitors.Store(ip, &visitor{limiter: limiter, lastSeen: time.Now()})
		return limiter
	}
	vv := v.(*visitor)
	vv.lastSeen = time.Now()
	return vv.limiter
}

func cleanupVisitors(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			visitors.Range(func(key, value interface{}) bool {
				v := value.(*visitor)
				if time.Since(v.lastSeen) > 3*time.Minute {
					visitors.Delete(key)
				}
				return true
			})
		}
	}()
}

// RateLimit middleware limits requests per IP.
// r = requests per second, b = burst size.
func RateLimit(r rate.Limit, b int) gin.HandlerFunc {
	cleanupVisitors(time.Minute)
	return func(c *gin.Context) {
		ip := c.ClientIP()
		limiter := getVisitor(ip, r, b)
		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			c.Abort()
			return
		}
		c.Next()
	}
}
