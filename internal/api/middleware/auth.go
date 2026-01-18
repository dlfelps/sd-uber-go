package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	UserIDKey   = "user_id"
	UserTypeKey = "user_type"

	UserTypeRider  = "rider"
	UserTypeDriver = "driver"
)

// MockAuth extracts user info from the Authorization header
// Format: "Bearer <user-id>" where user-id starts with "rider-" or "driver-"
func MockAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
			c.Abort()
			return
		}

		userID := parts[1]
		var userType string

		if strings.HasPrefix(userID, "rider-") {
			userType = UserTypeRider
		} else if strings.HasPrefix(userID, "driver-") {
			userType = UserTypeDriver
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid user id format"})
			c.Abort()
			return
		}

		c.Set(UserIDKey, userID)
		c.Set(UserTypeKey, userType)
		c.Next()
	}
}

// RequireRider ensures the authenticated user is a rider
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

// RequireDriver ensures the authenticated user is a driver
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

// GetUserID retrieves the user ID from context
func GetUserID(c *gin.Context) string {
	userID, _ := c.Get(UserIDKey)
	return userID.(string)
}

// GetUserType retrieves the user type from context
func GetUserType(c *gin.Context) string {
	userType, _ := c.Get(UserTypeKey)
	return userType.(string)
}
