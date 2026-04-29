package server

import (
    "log/slog"
    "net/http"
    "os"
    "strings"

    "github.com/gin-gonic/gin"

    "asika/common/config"
    "asika/common/models"
    "asika/daemon/handlers"
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

    // Load HTML templates
    engine.LoadHTMLGlob("daemon/templates/*.html")

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
           strings.HasPrefix(path, "/api/v1/wizard") ||
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

// setupRoutes configures routes
func (s *Server) setupRoutes() {
    // Health check
    s.engine.GET("/health", func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{"status": "ok"})
    })

    api := s.engine.Group("/api/v1")

    // Auth routes
    auth := api.Group("/auth")
    {
        auth.POST("/login", handlers.Login)
        auth.POST("/logout", handlers.Logout)
    }

    // Wizard routes
    wizard := api.Group("/wizard")
    {
        wizard.GET("", handlers.GetWizardSteps)
        wizard.POST("/step/:step", handlers.SubmitWizardStep)
    }

    // Protected routes (only if initialized)
    if s.cfg != nil {
        protected := api.Group("")
        protected.Use(RequireAuth())
        {
            users := protected.Group("/users")
            users.Use(RequireRole("admin"))
            {
                users.GET("", handlers.ListUsers)
                users.POST("", handlers.CreateUser)
                users.DELETE("/:username", handlers.DeleteUser)
            }

            prs := protected.Group("/repos/:repo_group/prs")
            prs.Use(RequireAnyRole("viewer", "operator", "admin"))
            {
                prs.GET("", handlers.ListPRs)
                prs.GET("/:pr_id", handlers.GetPR)
                prs.POST("/:pr_id/approve", handlers.ApprovePR)
                prs.POST("/:pr_id/close", handlers.ClosePR)
                prs.POST("/:pr_id/reopen", handlers.ReopenPR)
                prs.POST("/:pr_id/spam", handlers.MarkSpam)
                prs.DELETE("/:pr_id/spam", handlers.UnmarkSpam)
            }

            queue := protected.Group("/queue/:repo_group")
            queue.Use(RequireAnyRole("viewer", "operator", "admin"))
            {
                queue.GET("", handlers.GetQueue)
                queue.POST("/recheck", handlers.RecheckQueue)
            }

            rules := protected.Group("/rules")
            {
                rules.GET("/labels", handlers.GetLabelRules)
                rules.PUT("/labels", handlers.UpdateLabelRules)
            }

            sync := protected.Group("/sync")
            {
                sync.GET("/history", handlers.GetSyncHistory)
                sync.POST("/retry/:sync_id", handlers.RetrySync)
            }

            config := protected.Group("/config")
            config.Use(RequireRole("admin"))
            {
                config.GET("", handlers.GetConfig)
            }

            test := protected.Group("/test")
            test.Use(RequireRole("admin"))
            {
                test.POST("/notify", handlers.TestNotify)
            }
        }
    }

    // WebUI routes - use templates
    s.engine.GET("/", func(c *gin.Context) {
        c.HTML(http.StatusOK, "index.html", gin.H{"title": "Asika PR Manager"})
    })

    s.engine.GET("/wizard", func(c *gin.Context) {
        c.HTML(http.StatusOK, "wizard.html", gin.H{"title": "Setup Wizard"})
    })

    s.engine.GET("/dashboard", func(c *gin.Context) {
        c.HTML(http.StatusOK, "dashboard.html", gin.H{"title": "Dashboard"})
    })

    s.engine.GET("/login", func(c *gin.Context) {
        c.HTML(http.StatusOK, "login.html", gin.H{"title": "Login"})
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
