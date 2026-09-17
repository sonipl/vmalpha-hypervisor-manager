package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestDashboardDoesNotInventTelemetry(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, count := range []int64{10, 2, 3, 1, 0, 1, 3, 4, 2} {
		mock.ExpectQuery("SELECT count").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(count))
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/metrics", NewMonitoringHandler(db, logrus.New()).GetDashboardMetrics)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status: %d", recorder.Code)
	}
	var payload struct {
		VMs         map[string]int64    `json:"vms"`
		Hosts       map[string]int64    `json:"hosts"`
		Health      string              `json:"cluster_health"`
		Utilization map[string]*float64 `json:"utilization"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.VMs["migrating"] != 0 || payload.VMs["provisioning"] != 3 || payload.Hosts["ready"] != 2 {
		t.Fatal("Inventory states misreported")
	}
	if payload.Health != "Unknown" || len(payload.Utilization) != 4 {
		t.Fatal("Health or telemetry unavailable state missing")
	}
	for name, value := range payload.Utilization {
		if value != nil {
			t.Fatalf("Invented telemetry %s: %v", name, value)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDashboardDatabaseFailureIsNotZeroInventory(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT count").WillReturnError(errors.New("database unavailable"))
	router := gin.New()
	router.GET("/metrics", NewMonitoringHandler(db, logrus.New()).GetDashboardMetrics)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", recorder.Code)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
