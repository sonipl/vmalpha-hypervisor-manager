package handlers

import (
	"context"
	"encoding/json"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/novasphere/novasphere/internal/config"
	"github.com/novasphere/novasphere/internal/services/nativehost"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type networkRefusal struct{ calls int }

func (b *networkRefusal) Call(_ context.Context, r nativehost.Request) (json.RawMessage, error) {
	b.calls++
	return json.RawMessage(`{"supported":false}`), nil
}
func TestDistributedValidation(t *testing.T) {
	n := NewNativeHandler(nil, map[string]config.NativeHostConfig{"one": {}})
	base := DistributedSpec{Name: "Test", Bridge: "brtest", Uplink: "ens224", MTU: 1500, Hosts: []string{"one"}, PortGroups: []DistributedPortGroup{{"untagged", 0}, {"tagged", 42}}}
	if validateDistributedSpec(base, n) != nil {
		t.Fatal("valid spec rejected")
	}
	for _, change := range []func(*DistributedSpec){func(s *DistributedSpec) { s.Bridge = "bridge-name-too-long" }, func(s *DistributedSpec) { s.Uplink = "ens224;id" }, func(s *DistributedSpec) { s.Hosts = []string{"unknown"} }, func(s *DistributedSpec) { s.Hosts = []string{"one", "one"} }, func(s *DistributedSpec) { s.PortGroups = []DistributedPortGroup{{"bad", 4095}} }, func(s *DistributedSpec) { s.PortGroups = []DistributedPortGroup{{"one", 3}, {"two", 3}} }} {
		s := base
		change(&s)
		if validateDistributedSpec(s, n) == nil {
			t.Fatalf("accepted invalid spec %+v", s)
		}
	}
}
func TestDistributedReviewRefusesUnsupportedHostWithoutMutation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	n := NewNativeHandler(nil, map[string]config.NativeHostConfig{"one": {}})
	b := &networkRefusal{}
	n.Connect = func(config.NativeHostConfig) (nativeBroker, error) { return b, nil }
	h := NewDistributedNetworkHandler(nil, n)
	router := gin.New()
	router.POST("/review", h.Review)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("POST", "/review", strings.NewReader(`{"action":"create","spec":{"name":"Test","bridge":"brtest","uplink":"ens224","mtu":1500,"hosts":["one"],"port_groups":[]}}`)))
	if w.Code != 409 || b.calls != 1 {
		t.Fatalf("got %d calls %d: %s", w.Code, b.calls, w.Body)
	}
}
func TestDistributedApplyRequiresConfirmation(t *testing.T) {
	h := NewDistributedNetworkHandler(nil, nil)
	r := gin.New()
	r.POST("/apply", h.Apply)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/apply", strings.NewReader(`{"review_id":"anything","confirm":false}`)))
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
}

func TestDistributedApplyRejectsExpiredOrReplayedReview(t *testing.T) {
	for _, state := range []string{"expired", "applying"} {
		raw, mock, err := sqlmock.New()
		if err != nil {
			t.Fatal(err)
		}
		db, err := gorm.Open(postgres.New(postgres.Config{Conn: raw}), &gorm.Config{})
		if err != nil {
			t.Fatal(err)
		}
		expiry := time.Now().Add(time.Minute)
		storedState := "applying"
		if state == "expired" {
			expiry = time.Now().Add(-time.Minute)
			storedState = "reviewed"
		}
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT .*distributed_network_reviews").WillReturnRows(sqlmock.NewRows([]string{"id", "owner", "state", "expires_at"}).AddRow("review", "actor", storedState, expiry))
		mock.ExpectRollback()
		h := NewDistributedNetworkHandler(db, nil)
		r := gin.New()
		r.POST("/apply", func(c *gin.Context) { c.Set("user_id", "actor"); h.Apply(c) })
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("POST", "/apply", strings.NewReader(`{"review_id":"review","confirm":true}`)))
		if w.Code != 409 {
			t.Fatal(w.Code)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
		raw.Close()
	}
}
