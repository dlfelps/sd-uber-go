// Package middleware provides HTTP middleware for the Gin router.
//
// Go Learning Note — Middleware Pattern (Gin):
// In Gin, middleware is any function with the signature `gin.HandlerFunc`, which
// is `func(*gin.Context)`. Middleware functions form a chain: each one runs,
// optionally calls c.Next() to pass control to the next handler, and can call
// c.Abort() to stop the chain. This is the "chain of responsibility" pattern.
//
// Middleware is applied using .Use() on a router or route group. Common uses:
// authentication, logging, CORS headers, rate limiting, and request tracing.
//
// Go Learning Note — "github.com/gin-gonic/gin":
// Gin is one of Go's most popular HTTP frameworks. It wraps net/http with a
// fast router (radix tree based), JSON binding/validation, middleware support,
// and structured error handling. Alternatives include chi, echo, and fiber.
// For simpler APIs, the standard library's net/http with http.ServeMux (improved
// in Go 1.22) is often sufficient.
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Context keys for storing authenticated user data.
// These are used with c.Set()/c.Get() to pass data between middleware and handlers.
//
// Go Learning Note — Context Values:
// Gin's c.Set/c.Get stores request-scoped values in the *gin.Context. This is
// similar to the standard library's context.WithValue(). Use string keys (not
// raw strings) as constants to avoid typos and enable refactoring.
const (
	UserIDKey   = "user_id"
	UserTypeKey = "user_type"

	UserTypeRider  = "rider"
	UserTypeDriver = "driver"
)

// MockAuth extracts user info from the Authorization header.
// Format: "Bearer <user-id>" where user-id starts with "rider-" or "driver-".
//
// This is a simplified mock for the MVP. In production, you'd validate a real
// JWT token using a library like "github.com/golang-jwt/jwt/v5", verify the
// signature against a secret or public key, and extract claims from the token.
//
// Go Learning Note — Returning Functions (Closures):
// MockAuth() returns a gin.HandlerFunc — a function that returns a function.
// This pattern is common for middleware that needs configuration. The outer
// function (MockAuth) could accept parameters (like a JWT secret), and the
// inner function (the closure) captures those parameters. Here no config is
// needed, but the pattern is preserved for consistency with Gin's middleware API.
//
// Go Learning Note — c.Abort():
// c.Abort() prevents subsequent handlers in the chain from running. Without it,
// even after writing an error response, the next handler would still execute.
// Always pair error responses with c.Abort() in middleware.
func MockAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		// strings.SplitN splits into at most 2 parts, handling tokens with spaces.
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
			c.Abort()
			return
		}

		userID := parts[1]
		var userType string

		// Determine user type from the ID prefix — this mock approach avoids
		// needing a database lookup or token verification for the MVP.
		if strings.HasPrefix(userID, "rider-") {
			userType = UserTypeRider
		} else if strings.HasPrefix(userID, "driver-") {
			userType = UserTypeDriver
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id format"})
			c.Abort()
			return
		}

		// Store user info in the request context for downstream handlers.
		c.Set(UserIDKey, userID)
		c.Set(UserTypeKey, userType)
		c.Next() // Pass control to the next middleware/handler in the chain.
	}
}

// RequireRider is a role-based authorization middleware. It ensures the
// authenticated user is a rider. Must be used after MockAuth() in the chain.
func RequireRider() gin.HandlerFunc {
	return func(c *gin.Context) {
		userType, exists := c.Get(UserTypeKey)
		if !exists || userType != UserTypeRider {
			c.JSON(http.StatusForbidden, gin.H{"error": "rider access required"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireDriver ensures the authenticated user is a driver.
func RequireDriver() gin.HandlerFunc {
	return func(c *gin.Context) {
		userType, exists := c.Get(UserTypeKey)
		if !exists || userType != UserTypeDriver {
			c.JSON(http.StatusForbidden, gin.H{"error": "driver access required"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// GetUserID retrieves the user ID previously set by MockAuth middleware.
//
// Go Learning Note — Type Assertion:
// c.Get() returns (interface{}, bool). The .(string) is a type assertion that
// converts the interface{} to a concrete string. If the value isn't a string,
// this will panic at runtime. The safer form is `val, ok := x.(string)` which
// returns ok=false instead of panicking. Here the panic form is acceptable
// because this function should only be called after MockAuth middleware
// guarantees the value exists and is a string.
func GetUserID(c *gin.Context) string {
	userID, _ := c.Get(UserIDKey)
	return userID.(string)
}

// GetUserType retrieves the user type ("rider" or "driver") from context.
func GetUserType(c *gin.Context) string {
	userType, _ := c.Get(UserTypeKey)
	return userType.(string)
}
