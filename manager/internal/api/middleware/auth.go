package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/novasphere/novasphere/internal/models"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type Claims struct {
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
	Role     string    `json:"role"`
	TenantID uuid.UUID `json:"tenant_id"`
	Scope    string    `json:"scope"`
	jwt.RegisteredClaims
}

func AuthMiddleware(jwtSecret string, db *gorm.DB) gin.HandlerFunc {
	return authMiddleware(jwtSecret, func(ctx context.Context, id uuid.UUID) (models.User, error) {
		var user models.User
		if db == nil {
			return user, errors.New("authentication store unavailable")
		}
		err := db.WithContext(ctx).Preload("Role").First(&user, "id = ?", id).Error
		return user, err
	})
}

func authMiddleware(jwtSecret string, loadUser func(context.Context, uuid.UUID) (models.User, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization format, expected: Bearer <token>"})
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(parts[1], claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer("novasphere"), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			return
		}

		if claims.UserID == uuid.Nil || claims.Subject != claims.UserID.String() {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token identity"})
			return
		}
		// Account disablement and role/tenant changes take effect on the next request.
		user, err := loadUser(c.Request.Context(), claims.UserID)
		if err != nil || !user.IsActive || user.ID != claims.UserID || user.RoleID == uuid.Nil || user.Role.Name == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Account is unavailable"})
			return
		}
		c.Set("user_id", user.ID)
		c.Set("username", user.Username)
		c.Set("role", user.Role.Name)
		c.Set("tenant_id", user.TenantID)
		c.Set("scope", user.Role.Scope)

		c.Next()
	}
}

func RBACMiddleware(requiredResource, requiredVerb string) gin.HandlerFunc {
	return func(c *gin.Context) {
		value, exists := c.Get("role")
		role, validRole := value.(string)
		if !exists || !validRole {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "No role in context"})
			return
		}

		// Platform Admin bypasses all checks
		if role == "Platform Admin" {
			c.Next()
			return
		}

		scopeValue, _ := c.Get("scope")
		scope, validScope := scopeValue.(string)

		// Check if role has permission for resource+verb+scope
		if !validScope || !hasPermission(role, requiredResource, requiredVerb, scope) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Insufficient permissions",
				"required": map[string]string{
					"resource": requiredResource,
					"verb":     requiredVerb,
				},
			})
			return
		}

		c.Next()
	}
}

func AuditMiddleware(db *gorm.DB, log *logrus.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		// Don't audit GET requests unless they fail
		if c.Request.Method == "GET" && c.Writer.Status() < 400 {
			return
		}

		userID, _ := c.Get("user_id")
		username, _ := c.Get("username")

		uid, _ := userID.(uuid.UUID)
		uname, _ := username.(string)

		audit := models.AuditLog{
			Timestamp:  start,
			UserID:     uid,
			Username:   uname,
			Action:     c.Request.Method,
			Resource:   c.FullPath(),
			ResourceID: c.Param("id"),
			Details: models.JSONMap{
				"method":      c.Request.Method,
				"path":        c.Request.URL.Path,
				"status_code": c.Writer.Status(),
				"latency_ms":  time.Since(start).Milliseconds(),
				"user_agent":  c.Request.UserAgent(),
			},
			IPAddress: c.ClientIP(),
			Source:    detectSource(c),
			Success:   c.Writer.Status() < 400,
		}

		go func() {
			if err := db.Create(&audit).Error; err != nil {
				log.Errorf("Failed to write audit log: %v", err)
			}
		}()
	}
}

func CORSMiddleware(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		for _, allowed := range allowedOrigins {
			if allowed == "*" || allowed == origin {
				c.Header("Access-Control-Allow-Origin", origin)
				break
			}
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-Request-ID")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// ── Helpers ──

func hasPermission(role, resource, verb, scope string) bool {
	// Built-in role permission matrix
	permissions := map[string]map[string][]string{
		"Infrastructure Admin": {
			"hosts":           {"create", "read", "update", "delete", "list"},
			"clusters":        {"create", "read", "update", "delete", "list"},
			"storage":         {"create", "read", "update", "delete", "list"},
			"networks":        {"create", "read", "update", "delete", "list"},
			"virtualmachines": {"read", "list"},
		},
		"Security Admin": {
			"users":        {"create", "read", "update", "delete", "list"},
			"roles":        {"create", "read", "update", "delete", "list"},
			"audit":        {"read", "list"},
			"policies":     {"create", "read", "update", "delete", "list"},
			"certificates": {"create", "read", "update", "delete", "list"},
		},
		"Storage Admin": {
			"storage":   {"create", "read", "update", "delete", "list"},
			"volumes":   {"create", "read", "update", "delete", "list"},
			"backups":   {"create", "read", "update", "delete", "list"},
			"snapshots": {"create", "read", "update", "delete", "list"},
		},
		"Network Admin": {
			"networks":  {"create", "read", "update", "delete", "list"},
			"firewalls": {"create", "read", "update", "delete", "list"},
			"dns":       {"create", "read", "update", "delete", "list"},
		},
		"VM Operator": {
			"virtualmachines": {"create", "read", "update", "delete", "list", "console", "migrate", "snapshot"},
			"templates":       {"read", "list"},
			"networks":        {"read", "list"},
			"storage":         {"read", "list"},
		},
		"VM User": {
			"virtualmachines": {"read", "list", "console", "start", "stop"},
		},
		"Viewer": {
			"virtualmachines": {"read", "list"},
			"hosts":           {"read", "list"},
			"metrics":         {"read"},
		},
	}

	rolePerm, exists := permissions[role]
	if !exists {
		return false
	}

	verbs, exists := rolePerm[resource]
	if !exists {
		return false
	}

	for _, v := range verbs {
		if v == verb {
			return true
		}
	}
	return false
}

func detectSource(c *gin.Context) string {
	if c.GetHeader("X-AI-Assistant") == "true" {
		return "ai-assistant"
	}
	if c.GetHeader("X-Automation") == "true" {
		return "automation"
	}
	ua := c.Request.UserAgent()
	if strings.Contains(ua, "novactl") {
		return "cli"
	}
	return "ui"
}
