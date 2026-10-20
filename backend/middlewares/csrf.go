package middlewares

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"tripledger/config"
)

// CSRFProtectionMiddleware enforces:
// 1. Requirement of header X-Requested-With: Splito on all mutating methods (POST, PUT, DELETE, PATCH).
// 2. Verification of Origin (or Referer) against the allowed origins list.
// Safe methods (GET, HEAD, OPTIONS) are exempt.
func CSRFProtectionMiddleware(cfg *config.Config) gin.HandlerFunc {
	allowedList := strings.Split(cfg.AllowedOrigins, ",")
	allowedMap := make(map[string]bool)
	for _, o := range allowedList {
		trimmed := strings.TrimSpace(o)
		if trimmed != "" {
			allowedMap[strings.ToLower(trimmed)] = true
		}
	}

	return func(c *gin.Context) {
		method := c.Request.Method
		// Skip safe methods
		if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
			c.Next()
			return
		}

		// 1. Enforce X-Requested-With: Splito
		xReq := c.GetHeader("X-Requested-With")
		if xReq != "Splito" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Forbidden: missing or invalid X-Requested-With header",
			})
			return
		}

		// 2. Check Origin / Referer against allowed origins (if present)
		origin := c.GetHeader("Origin")
		sourceURL := origin
		if sourceURL == "" {
			sourceURL = c.GetHeader("Referer")
		}

		if sourceURL != "" {
			parsed, err := url.Parse(sourceURL)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error": "Forbidden: invalid origin or referer URL",
				})
				return
			}
			originOnly := strings.ToLower(parsed.Scheme + "://" + parsed.Host)
			if !allowedMap["*"] && !allowedMap[originOnly] && !strings.HasSuffix(strings.ToLower(parsed.Host), ".vercel.app") {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error": "Forbidden: untrusted origin: " + originOnly,
				})
				return
			}
		}

		c.Next()
	}
}

// CORSMiddleware sets exact origin allowlist and credentials (never '*')
func CORSMiddleware(cfg *config.Config) gin.HandlerFunc {
	allowedList := strings.Split(cfg.AllowedOrigins, ",")
	allowedMap := make(map[string]bool)
	for _, o := range allowedList {
		trimmed := strings.TrimSpace(o)
		if trimmed != "" {
			allowedMap[strings.ToLower(trimmed)] = true
		}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			parsed, err := url.Parse(origin)
			if err == nil {
				originOnly := strings.ToLower(parsed.Scheme + "://" + parsed.Host)
				if allowedMap["*"] || allowedMap[originOnly] || strings.HasSuffix(strings.ToLower(parsed.Host), ".vercel.app") {
					c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				}
			}
		} else if len(allowedList) > 0 {
			// default to first allowed origin if no Origin header
			c.Writer.Header().Set("Access-Control-Allow-Origin", strings.TrimSpace(allowedList[0]))
		}

		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, Bypass-Tunnel-Reminder")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
