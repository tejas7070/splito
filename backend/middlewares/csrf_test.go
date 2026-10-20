package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"tripledger/config"
)

func setupTestRouter(cfg *config.Config) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORSMiddleware(cfg))
	r.Use(CSRFProtectionMiddleware(cfg))

	r.GET("/api/test-get", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.POST("/api/test-post", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "post-ok"})
	})

	return r
}

func TestCSRFProtection(t *testing.T) {
	cfg := &config.Config{
		AllowedOrigins: "http://localhost:8080,https://app.splito.in",
	}
	r := setupTestRouter(cfg)

	// Test 1: GET endpoint is unaffected even without headers
	{
		req, _ := http.NewRequest(http.MethodGet, "/api/test-get", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("GET: expected 200, got %d", w.Code)
		}
	}

	// Test 2: POST without X-Requested-With header returns 403 Forbidden
	{
		req, _ := http.NewRequest(http.MethodPost, "/api/test-post", nil)
		req.Header.Set("Origin", "http://localhost:8080")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("POST without X-Requested-With: expected 403, got %d", w.Code)
		}
	}

	// Test 3: POST with wrong Origin returns 403 Forbidden
	{
		req, _ := http.NewRequest(http.MethodPost, "/api/test-post", nil)
		req.Header.Set("X-Requested-With", "Splito")
		req.Header.Set("Origin", "http://malicious-site.com")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusForbidden {
			t.Errorf("POST with untrusted Origin: expected 403, got %d", w.Code)
		}
	}

	// Test 4: Normal app request with X-Requested-With: Splito and allowed Origin succeeds
	{
		req, _ := http.NewRequest(http.MethodPost, "/api/test-post", nil)
		req.Header.Set("X-Requested-With", "Splito")
		req.Header.Set("Origin", "http://localhost:8080")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Valid POST: expected 200, got %d", w.Code)
		}
	}

	// Test 5: Normal app request with Referer from allowed origin succeeds
	{
		req, _ := http.NewRequest(http.MethodPost, "/api/test-post", nil)
		req.Header.Set("X-Requested-With", "Splito")
		req.Header.Set("Referer", "https://app.splito.in/dashboard")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Valid POST with Referer: expected 200, got %d", w.Code)
		}
	}
}
