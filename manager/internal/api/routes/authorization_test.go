package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/novasphere/novasphere/internal/config"
	"github.com/sirupsen/logrus"
)

func TestManagementRoutesRejectNonAdministrators(t *testing.T) {
	gin.SetMode(gin.TestMode)
	paths := []struct{ method, path string }{
		{"GET", "/native/hosts/example/files"}, {"GET", "/native/hosts/example/metrics"}, {"GET", "/native/hosts"}, {"GET", "/native/hosts/example/inventory"}, {"POST", "/native/hosts/example/operations"}, {"GET", "/native/tasks/example"},
		{"GET", "/ai/recommendations"}, {"POST", "/ai/recommendations/example/apply"}, {"POST", "/ai/recommendations/example/dismiss"},
		{"GET", "/ai/insights"}, {"POST", "/ai/chat"}, {"POST", "/ai/migration/assess"},
		{"GET", "/ai/policies"}, {"POST", "/ai/policies"}, {"PUT", "/ai/policies/example"},
		{"GET", "/setup/status"}, {"POST", "/setup/nodes"}, {"POST", "/setup/nodes/example/join-k8s"},
		{"POST", "/setup/ceph/hosts"}, {"POST", "/setup/ceph/enable"}, {"DELETE", "/setup/nodes/example"},
		{"GET", "/settings/ldap"}, {"PUT", "/settings/ldap"}, {"POST", "/settings/ldap/test"},
		{"POST", "/monitoring/alerts"}, {"PUT", "/monitoring/alerts/example"}, {"DELETE", "/monitoring/alerts/example"},
	}
	for _, role := range []string{"Viewer", "VM User", "VM Operator", "Infrastructure Admin", "Security Admin"} {
		r := setupWithMiddleware(&config.Config{}, nil, nil, logrus.New(), func(c *gin.Context) { c.Set("role", role); c.Set("scope", "Global"); c.Next() }, func(c *gin.Context) { c.Next() })
		for _, p := range paths {
			t.Run(role+"/"+p.method+p.path, func(t *testing.T) {
				req := httptest.NewRequest(p.method, "/api/v1"+p.path, strings.NewReader("{}"))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				if w.Code != http.StatusForbidden {
					t.Fatalf("expected denial before handler/database access; got %d: %s", w.Code, w.Body.String())
				}
			})
		}
	}
}
