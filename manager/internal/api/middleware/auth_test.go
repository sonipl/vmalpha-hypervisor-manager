package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/novasphere/novasphere/internal/models"
)

const testSecret = "test-only-signing-key-not-a-production-secret"

func testAccount() models.User {
	roleID := uuid.New()
	return models.User{BaseModel: models.BaseModel{ID: uuid.New()}, Username: "current-user", IsActive: true,
		RoleID: roleID, TenantID: uuid.New(), Role: models.Role{BaseModel: models.BaseModel{ID: roleID}, Name: "Viewer", Scope: "Global"}}
}

func testClaims(user models.User) Claims {
	return Claims{UserID: user.ID, Username: "old-name", Role: "Platform Admin", TenantID: uuid.New(), Scope: "Global",
		RegisteredClaims: jwt.RegisteredClaims{Issuer: "novasphere", Subject: user.ID.String(),
			IssuedAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)), ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
}

func requestWithToken(t *testing.T, user models.User, claims Claims, method jwt.SigningMethod, lookupErr error, handlers ...gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(authMiddleware(testSecret, func(context.Context, uuid.UUID) (models.User, error) { return user, lookupErr }))
	r.GET("/private", handlers...)
	token, err := jwt.NewWithClaims(method, claims).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/private", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestCurrentAccountOverridesStaleToken(t *testing.T) {
	u := testAccount()
	w := requestWithToken(t, u, testClaims(u), jwt.SigningMethodHS256, nil, func(c *gin.Context) {
		if c.GetString("role") != "Viewer" || c.GetString("username") != u.Username || c.GetString("scope") != u.Role.Scope {
			t.Error("stale token authorization used")
		}
		if c.MustGet("tenant_id") != u.TenantID {
			t.Error("stale token tenant used")
		}
		c.Status(http.StatusNoContent)
	})
	if w.Code != http.StatusNoContent {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	// A token issued as administrator must lose write access after demotion.
	w = requestWithToken(t, u, testClaims(u), jwt.SigningMethodHS256, nil, RBACMiddleware("hosts", "delete"), func(c *gin.Context) { c.Status(http.StatusNoContent) })
	if w.Code != http.StatusForbidden {
		t.Fatalf("demoted administrator status %d", w.Code)
	}
}

func TestAuthenticationFailsClosed(t *testing.T) {
	cases := []struct {
		name   string
		alter  func(*models.User, *Claims)
		method jwt.SigningMethod
		err    error
	}{
		{"disabled", func(u *models.User, c *Claims) { u.IsActive = false }, jwt.SigningMethodHS256, nil},
		{"deleted-or-store-failure", nil, jwt.SigningMethodHS256, errors.New("unavailable")},
		{"missing-role", func(u *models.User, c *Claims) { u.Role.Name = "" }, jwt.SigningMethodHS256, nil},
		{"wrong-algorithm", nil, jwt.SigningMethodHS512, nil},
		{"wrong-issuer", func(u *models.User, c *Claims) { c.Issuer = "other" }, jwt.SigningMethodHS256, nil},
		{"missing-expiry", func(u *models.User, c *Claims) { c.ExpiresAt = nil }, jwt.SigningMethodHS256, nil},
		{"expired", func(u *models.User, c *Claims) { c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute)) }, jwt.SigningMethodHS256, nil},
		{"future-issued", func(u *models.User, c *Claims) { c.IssuedAt = jwt.NewNumericDate(time.Now().Add(time.Hour)) }, jwt.SigningMethodHS256, nil},
		{"wrong-subject", func(u *models.User, c *Claims) { c.Subject = uuid.NewString() }, jwt.SigningMethodHS256, nil},
		{"empty-identity", func(u *models.User, c *Claims) { c.UserID = uuid.Nil }, jwt.SigningMethodHS256, nil},
		{"wrong-loaded-user", func(u *models.User, c *Claims) { u.ID = uuid.New() }, jwt.SigningMethodHS256, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u := testAccount()
			c := testClaims(u)
			if tc.alter != nil {
				tc.alter(&u, &c)
			}
			w := requestWithToken(t, u, c, tc.method, tc.err, func(c *gin.Context) { t.Error("unauthorized request reached handler") })
			if w.Code != http.StatusUnauthorized {
				t.Fatalf("status %d", w.Code)
			}
		})
	}
}

func TestRBACInvalidContextDoesNotPanic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("role", 123) })
	r.GET("/private", RBACMiddleware("hosts", "read"), func(c *gin.Context) { t.Error("invalid context authorized") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/private", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("status %d", w.Code)
	}
}
