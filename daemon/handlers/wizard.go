package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "log/slog"

    "asika/common/config"
)

// GetWizardSteps returns the wizard steps
func GetWizardSteps(c *gin.Context) {
    // Check if already initialized
    if config.IsInitialized(config.ConfigPath) {
        c.JSON(http.StatusBadRequest, gin.H{"error": "already initialized", "code": 400})
        return
    }

    steps := config.GetWizardSteps()
    c.JSON(http.StatusOK, steps)
}

// SubmitWizardStep submits a wizard step
func SubmitWizardStep(c *gin.Context) {
    step := c.Param("step")

    var data map[string]interface{}
    if err := c.ShouldBindJSON(&data); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request", "code": 400})
        return
    }

    slog.Info("wizard step submitted", "step", step, "data", data)

    c.JSON(http.StatusOK, gin.H{"message": "step saved", "step": step})
}

// WizardPage renders the wizard page
func WizardPage(c *gin.Context) {
    c.HTML(http.StatusOK, "wizard.html", gin.H{"title": "Asika Setup Wizard"})
}

// LoginPage renders the login page
func LoginPage(c *gin.Context) {
    c.HTML(http.StatusOK, "login.html", gin.H{"title": "Login - Asika"})
}

// Dashboard renders the dashboard
func Dashboard(c *gin.Context) {
    c.HTML(http.StatusOK, "dashboard.html", gin.H{"title": "Dashboard - Asika"})
}

// PRListPage renders the PR list page
func PRListPage(c *gin.Context) {
    repoGroup := c.Param("repo_group")
    c.HTML(http.StatusOK, "pr_list.html", gin.H{
        "title":     "PRs - Asika",
        "repo_group": repoGroup,
    })
}

// PRDetailPage renders the PR detail page
func PRDetailPage(c *gin.Context) {
    repoGroup := c.Param("repo_group")
    prID := c.Param("pr_id")
    c.HTML(http.StatusOK, "pr_detail.html", gin.H{
        "title":      "PR Detail - Asika",
        "repo_group": repoGroup,
        "pr_id":      prID,
    })
}

// QueuePage renders the queue page
func QueuePage(c *gin.Context) {
    repoGroup := c.Param("repo_group")
    c.HTML(http.StatusOK, "queue.html", gin.H{
        "title":      "Queue - Asika",
        "repo_group": repoGroup,
    })
}
