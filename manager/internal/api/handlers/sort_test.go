package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestInventorySort(t *testing.T) {
	for _, tc := range []struct {
		field, direction, column string
		desc                     bool
	}{
		{"", "", "name", false}, {"name", "DESC", "name", true},
		{"-created_at", "", "created_at", true}, {"memory_mb", "asc", "memory_mb", false},
	} {
		got, err := validatedSort(tc.field, tc.direction, vmSortFields)
		if err != nil || got.Column.Name != tc.column || got.Column.Raw || got.Desc != tc.desc {
			t.Fatalf("unexpected sort for %q/%q: %#v %v", tc.field, tc.direction, got, err)
		}
	}
}

func TestInventoryRejectsSQLSortBeforeDatabaseAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct{ field, direction string }{
		{"name; DROP TABLE users", "asc"}, {"(SELECT pg_sleep(10))", ""},
		{"name", "desc; SELECT 1"}, {"name", "asc NULLS FIRST"},
		{"unknown", ""}, {"-created_at", "asc"},
	} {
		for _, kind := range []string{"hosts", "vms"} {
			t.Run(kind+"/"+tc.field+"/"+tc.direction, func(t *testing.T) {
				r := gin.New()
				// Nil dependencies make any premature sync/query access fail the test.
				if kind == "hosts" {
					r.GET("/list", (&HostHandler{}).ListHosts)
				} else {
					r.GET("/list", (&VMHandler{}).ListVMs)
				}
				q := url.Values{"sort": {tc.field}, "order": {tc.direction}}
				w := httptest.NewRecorder()
				r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/list?"+q.Encode(), nil))
				if w.Code != http.StatusBadRequest {
					t.Fatalf("status %d", w.Code)
				}
			})
		}
	}
}
