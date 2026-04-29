package handlers

import (
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "log/slog"

    bolt "go.etcd.io/bbolt"

    "asika/common/auth"
    "asika/common/db"
    "asika/common/models"
)

// Login handles user login
func Login(c *gin.Context) {
    var req struct {
        Username string `json:"username"`
        Password string `json:"password"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request", "code": 400})
        return
    }

    // Get user from DB
    var user models.User
    err := db.View(func(tx *bolt.Tx) error {
        b := tx.Bucket([]byte(db.BucketUsers))
        if b == nil {
            return fmt.Errorf("bucket not found")
        }
        data := b.Get([]byte(req.Username))
        if data == nil {
            return fmt.Errorf("user not found")
        }
        return json.Unmarshal(data, &user)
    })

    if err != nil {
        slog.Warn("login failed: user not found", "username", req.Username)
        c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials", "code": 401})
        return
    }

    // Check password
    if !auth.CheckPassword(req.Password, user.PasswordHash) {
        slog.Warn("login failed: wrong password", "username", req.Username)
        c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials", "code": 401})
        return
    }

    // Generate token
    token, err := auth.GenerateJWT(user.Username, user.Role)
    if err != nil {
        slog.Error("failed to generate token", "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "code": 500})
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "token": token,
        "user": gin.H{
            "username": user.Username,
            "role":     user.Role,
        },
    })
}

// Logout handles user logout
func Logout(c *gin.Context) {
    token := extractToken(c)
    if token != "" {
        auth.BlacklistToken(token)
    }
    c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

// ListUsers lists all users (admin only)
func ListUsers(c *gin.Context) {
    users := make([]models.User, 0)

    err := db.ForEach(db.BucketUsers, func(key, value []byte) error {
        var user models.User
        if err := json.Unmarshal(value, &user); err != nil {
            return err
        }
        // Don't return password hash
        user.PasswordHash = ""
        users = append(users, user)
        return nil
    })

    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list users", "code": 500})
        return
    }

    c.JSON(http.StatusOK, users)
}

// CreateUser creates a new user (admin only)
func CreateUser(c *gin.Context) {
    var req struct {
        Username string `json:"username"`
        Password string `json:"password"`
        Role     string `json:"role"`
    }

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request", "code": 400})
        return
    }

    // Validate role
    if req.Role != "admin" && req.Role != "operator" && req.Role != "viewer" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role", "code": 400})
        return
    }

    // Hash password
    hash, err := auth.HashPassword(req.Password)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password", "code": 500})
        return
    }

    user := models.User{
        Username:     req.Username,
        PasswordHash: hash,
        Role:         req.Role,
        CreatedAt:    time.Now(),
    }

    data, err := json.Marshal(user)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error", "code": 500})
        return
    }

    if err := db.Put(db.BucketUsers, req.Username, data); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user", "code": 500})
        return
    }

    c.JSON(http.StatusCreated, gin.H{"message": "user created"})
}

// DeleteUser deletes a user (admin only)
func DeleteUser(c *gin.Context) {
    username := c.Param("username")

    if err := db.Delete(db.BucketUsers, username); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete user", "code": 500})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "user deleted"})
}

// extractToken extracts the token from Authorization header
func extractToken(c *gin.Context) string {
    // Implementation from auth middleware
    return ""
}
