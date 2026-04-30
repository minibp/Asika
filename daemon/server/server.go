package server

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"asika/common/config"
	"asika/common/models"
	"asika/daemon/handlers"
	"asika/daemon/templates"
)

// Server represents the HTTP server
type Server struct {
	engine *gin.Engine
	cfg    *models.Config
}

// NewServer creates a new server instance
func NewServer(cfg *models.Config) *Server {
	if cfg != nil && cfg.Server.Mode == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()

	// Load HTML templates from embedded FS
	t, err := template.ParseFS(templates.FS, "*.html")
	if err != nil {
		panic(fmt.Sprintf("failed to parse templates: %v", err))
	}
	engine.SetHTMLTemplate(t)

	s := &Server{
		cfg:    cfg,
		engine: engine,
	}

	s.setupMiddleware()
	s.setupRoutes()

	return s
}

// initCheckMiddleware redirects to wizard if not initialized
func initCheckMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path

		// Skip check for wizard, login, and health paths
		if strings.HasPrefix(path, "/wizard") ||
			strings.HasPrefix(path, "/api/v1/auth") ||
			strings.HasPrefix(path, "/login") ||
			path == "/health" {
			c.Next()
			return
		}

		// Check if config exists
		configPath := os.Getenv("ASIKA_CONFIG")
		if configPath == "" {
			configPath = "/etc/asika_config.toml"
		}

		if !config.IsInitialized(configPath) {
			if strings.HasPrefix(path, "/api/") {
				c.JSON(http.StatusServiceUnavailable, gin.H{
					"error": "server not initialized",
					"code":  503,
				})
			} else {
				c.Redirect(http.StatusFound, "/wizard")
			}
			c.Abort()
			return
		}

		c.Next()
	}
}

// setupMiddleware configures middleware
func (s *Server) setupMiddleware() {
	s.engine.Use(initCheckMiddleware())
	s.engine.Use(Logger())
	s.engine.Use(gin.Recovery())

	if s.cfg != nil {
		s.engine.Use(AuthMiddleware())
	}
}

// setupRoutes configures routes according to tasks.md section 8
func (s *Server) setupRoutes() {
	// Health check
	s.engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	api := s.engine.Group("/api/v1")

	// Authentication routes (8.1)
	auth := api.Group("/auth")
	{
		auth.POST("/login", handlers.Login)
		auth.POST("/logout", handlers.Logout)
	}

	// Wizard routes (10. WebUI Wizard)
	wizard := api.Group("/wizard")
	{
		wizard.GET("", handlers.GetWizardSteps)
		wizard.POST("/step/complete", handlers.CompleteWizard)
		wizard.POST("/step/:step", handlers.SubmitWizardStep)
	}

	// Protected routes (require JWT)
	protected := api.Group("")
	protected.Use(RequireAuth())
	{
		// User management (8.1)
		users := protected.Group("/users")
		users.Use(RequireRole("admin"))
		{
			users.GET("", handlers.ListUsers)
			users.POST("", handlers.CreateUser)
			users.DELETE("/:username", handlers.DeleteUser)
		}

		// PR management (8.2)
		prs := protected.Group("/repos/:repo_group/prs")
		prs.Use(RequireAnyRole("viewer", "operator", "admin"))
		{
			prs.GET("", handlers.ListPRs)
			prs.GET("/:pr_id", handlers.GetPR)
			prs.POST("/:pr_id/approve", handlers.ApprovePR)
			prs.POST("/:pr_id/close", handlers.ClosePR)
			prs.POST("/:pr_id/reopen", handlers.ReopenPR)
			prs.POST("/:pr_id/spam", handlers.MarkSpam)
		}

		// Queue management (8.3)
		queue := protected.Group("/queue/:repo_group")
		queue.Use(RequireAnyRole("viewer", "operator", "admin"))
		{
			queue.GET("", handlers.GetQueue)
			queue.POST("/recheck", handlers.RecheckQueue)
		}

		// Audit logs (8.2)
		logs := protected.Group("/logs")
		logs.Use(RequireAnyRole("viewer", "operator", "admin"))
		{
			logs.GET("", handlers.GetLogs)
		}

		// Config management (8.4)
		config := protected.Group("/config")
		config.Use(RequireRole("admin"))
		{
			config.GET("", handlers.GetConfig)
			config.PUT("", handlers.UpdateConfig)
		}

		// Sync history (8.5)
		sync := protected.Group("/sync")
		sync.Use(RequireAnyRole("viewer", "operator", "admin"))
		{
			sync.GET("/history", handlers.GetSyncHistory)
			sync.POST("/retry/:sync_id", handlers.RetrySync)
		}

		// Test notification (8.6)
		test := protected.Group("/test")
		test.Use(RequireRole("admin"))
		{
			test.POST("/notify", handlers.TestNotify)
		}
	}

	// WebUI routes - server-rendered (SSR) per tasks.md 2.3
	s.engine.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{"title": "Asika PR Manager"})
	})

	s.engine.GET("/wizard", func(c *gin.Context) {
		c.HTML(http.StatusOK, "wizard.html", gin.H{"title": "Setup Wizard - Asika"})
	})

	s.engine.GET("/login", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.html", gin.H{"title": "Login - Asika"})
	})

	s.engine.GET("/dashboard", func(c *gin.Context) {
		user := c.GetString("username")
		c.HTML(http.StatusOK, "dashboard.html", gin.H{
			"title":          "Dashboard - Asika",
			"authenticated": true,
			"username":       user,
		})
	})

	s.engine.GET("/prs", func(c *gin.Context) {
		user := c.GetString("username")
		c.HTML(http.StatusOK, "pr_list.html", gin.H{
			"title":          "PRs - Asika",
			"authenticated": true,
			"username":       user,
			"repo_group":     "main",
		})
	})

	s.engine.GET("/prs/:pr_id", func(c *gin.Context) {
		user := c.GetString("username")
		c.HTML(http.StatusOK, "pr_detail.html", gin.H{
			"title":          "PR Detail - Asika",
			"authenticated": true,
			"username":       user,
			"repo_group":     "main",
			"pr_id":          c.Param("pr_id"),
		})
	})

	s.engine.GET("/queue", func(c *gin.Context) {
		user := c.GetString("username")
		c.HTML(http.StatusOK, "queue.html", gin.H{
			"title":          "Queue - Asika",
			"authenticated": true,
			"username":       user,
			"repo_group":     "main",
		})
	})

	s.engine.GET("/config", func(c *gin.Context) {
		user := c.GetString("username")
		c.HTML(http.StatusOK, "config.html", gin.H{
			"title":          "Config - Asika",
			"authenticated": true,
			"username":       user,
		})
	})

	// Webhook routes (no auth)
	s.engine.POST("/webhook/:repo_group/:platform", handlers.WebhookHandler)
}

// Start starts the server
func (s *Server) Start() error {
	if s.cfg == nil {
		slog.Info("starting server in initialization mode", "listen", ":8080")
		return s.engine.Run(":8080")
	}
	slog.Info("starting server", "listen", s.cfg.Server.Listen)
	return s.engine.Run(s.cfg.Server.Listen)
}

// Stop stops the server
func (s *Server) Stop() error {
	return nil
}
