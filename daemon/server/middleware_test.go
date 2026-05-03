package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSplitToken(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"Bearer token123", []string{"Bearer", "token123"}},
		{"token456", nil}, // No space, return nil
		{"", nil},
		{"Bearer ", []string{"Bearer", ""}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitToken(tt.input)
			if tt.want == nil {
				if got != nil {
					t.Errorf("splitToken(%q) = %v, want nil", tt.input, got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("splitToken(%q) length = %d, want %d", tt.input, len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitToken(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExtractToken_FromHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		Header: make(http.Header),
	}
	c.Request.Header.Set("Authorization", "Bearer test-token")

	token := extractToken(c)
	if token != "test-token" {
		t.Errorf("extractToken() = %q, want test-token", token)
	}
}

func TestExtractToken_FromCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		Header: make(http.Header),
	}
	// No Authorization header, but has cookie
	c.Request.AddCookie(&http.Cookie{
		Name:  "asika_token",
		Value: "cookie-token",
	})

	token := extractToken(c)
	if token != "cookie-token" {
		t.Errorf("extractToken() = %q, want cookie-token", token)
	}
}

func TestExtractToken_NotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = &http.Request{
		Header: make(http.Header),
	}

	token := extractToken(c)
	if token != "" {
		t.Errorf("extractToken() = %q, want empty string", token)
	}
}
