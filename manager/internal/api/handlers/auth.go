package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/novasphere/novasphere/internal/api/middleware"
	"github.com/novasphere/novasphere/internal/models"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthHandler struct {
	DB        *gorm.DB
	Log       *logrus.Logger
	JWTSecret string
	TokenExp  int // hours
}

func NewAuthHandler(db *gorm.DB, log *logrus.Logger, jwtSecret string, tokenExp int) *AuthHandler {
	return &AuthHandler{DB: db, Log: log, JWTSecret: jwtSecret, TokenExp: tokenExp}
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      UserInfo  `json:"user"`
}

type UserInfo struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Email    string    `json:"email"`
	FullName string    `json:"full_name"`
	Role     string    `json:"role"`
	TenantID uuid.UUID `json:"tenant_id"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := h.DB.Preload("Role").Preload("Tenant").
		Where("username = ? AND is_active = true", req.Username).
		First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Generate JWT
	expiresAt := time.Now().Add(time.Duration(h.TokenExp) * time.Hour)
	claims := &middleware.Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role.Name,
		TenantID: user.TenantID,
		Scope:    user.Role.Scope,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "novasphere",
			Subject:   user.ID.String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(h.JWTSecret))
	if err != nil {
		h.Log.Errorf("Failed to sign JWT: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Update last login
	now := time.Now()
	h.DB.Model(&user).Update("last_login", &now)

	c.JSON(http.StatusOK, LoginResponse{
		Token:     tokenStr,
		ExpiresAt: expiresAt,
		User: UserInfo{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			FullName: user.FullName,
			Role:     user.Role.Name,
			TenantID: user.TenantID,
		},
	})
}

func (h *AuthHandler) GetProfile(c *gin.Context) {
	userID, _ := c.Get("user_id")
	var user models.User
	if err := h.DB.Preload("Role").Preload("Tenant").First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	c.JSON(http.StatusOK, UserInfo{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		FullName: user.FullName,
		Role:     user.Role.Name,
		TenantID: user.TenantID,
	})
}

func (h *AuthHandler) ListUsers(c *gin.Context) {
	var users []models.User
	if err := h.DB.WithContext(c.Request.Context()).Preload("Role").Preload("Tenant").Find(&users).Error; err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "User records unavailable"})
		return
	}

	var result []UserInfo
	for _, u := range users {
		result = append(result, UserInfo{
			ID: u.ID, Username: u.Username, Email: u.Email,
			FullName: u.FullName, Role: u.Role.Name, TenantID: u.TenantID,
		})
	}
	c.JSON(http.StatusOK, result)
}

func (h *AuthHandler) CreateUser(c *gin.Context) {
	var req struct {
		Username string    `json:"username" binding:"required"`
		Email    string    `json:"email" binding:"required,email"`
		FullName string    `json:"full_name" binding:"required"`
		Password string    `json:"password" binding:"required,min=8"`
		RoleID   uuid.UUID `json:"role_id" binding:"required"`
		TenantID uuid.UUID `json:"tenant_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	user := models.User{
		Username: req.Username,
		Email:    req.Email,
		FullName: req.FullName,
		Password: string(hashed),
		RoleID:   req.RoleID,
		TenantID: req.TenantID,
		IsActive: true,
		Source:   "local",
	}

	if err := h.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Username or email already exists"})
		return
	}

	c.JSON(http.StatusCreated, UserInfo{
		ID: user.ID, Username: user.Username, Email: user.Email,
		FullName: user.FullName, TenantID: user.TenantID,
	})
}

// Roles
func (h *AuthHandler) ListRoles(c *gin.Context) {
	var roles []models.Role
	if err := h.DB.WithContext(c.Request.Context()).Find(&roles).Error; err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Roles unavailable"})
		return
	}
	c.JSON(http.StatusOK, roles)
}

func (h *AuthHandler) CreateRole(c *gin.Context) {
	var role models.Role
	if err := c.ShouldBindJSON(&role); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.DB.Create(&role).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create role"})
		return
	}
	c.JSON(http.StatusCreated, role)
}

// Tenants
func (h *AuthHandler) ListTenants(c *gin.Context) {
	var tenants []models.Tenant
	if err := h.DB.WithContext(c.Request.Context()).Find(&tenants).Error; err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Tenants unavailable"})
		return
	}
	c.JSON(http.StatusOK, tenants)
}

func (h *AuthHandler) CreateTenant(c *gin.Context) {
	var tenant models.Tenant
	if err := c.ShouldBindJSON(&tenant); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.DB.Create(&tenant).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create tenant"})
		return
	}
	c.JSON(http.StatusCreated, tenant)
}

// Audit Logs
func (h *AuthHandler) ListAuditLogs(c *gin.Context) {
	var logs []models.AuditLog
	query := h.DB.WithContext(c.Request.Context()).Model(&models.AuditLog{}).Order("timestamp DESC")

	if userID := c.Query("user_id"); userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	if action := c.Query("action"); action != "" {
		query = query.Where("action = ?", action)
	}
	if resource := c.Query("resource"); resource != "" {
		query = query.Where("resource ILIKE ?", "%"+resource+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Audit records unavailable"})
		return
	}
	page, perPage := parsePagination(c)
	if err := query.Offset((page - 1) * perPage).Limit(perPage).Find(&logs).Error; err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Audit records unavailable"})
		return
	}

	c.JSON(http.StatusOK, PaginatedResponse{Data: logs, Total: total, Page: page, PerPage: perPage})
}
