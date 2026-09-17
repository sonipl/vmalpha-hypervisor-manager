package database

import (
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"strings"
	"testing"
)

func TestBootstrapRequiresOperatorPassword(t *testing.T) {
	for _, password := range []string{"", "short", strings.Repeat("x", 73)} {
		t.Run("invalid-length", func(t *testing.T) {
			t.Setenv("NOVA_BOOTSTRAP_PASSWORD", password)
			db, mock := bootstrapDB(t)
			mock.ExpectQuery("SELECT count").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			if err := SeedBootstrap(db); err == nil || !strings.Contains(err.Error(), "NOVA_BOOTSTRAP_PASSWORD") {
				t.Fatal("missing first-start password validation")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestBootstrapDoesNotResetExistingUsers(t *testing.T) {
	t.Setenv("NOVA_BOOTSTRAP_PASSWORD", "")
	db, mock := bootstrapDB(t)
	mock.ExpectQuery("SELECT count").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	if err := SeedBootstrap(db); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestBootstrapRollsBackOnFailure(t *testing.T) {
	t.Setenv("NOVA_BOOTSTRAP_PASSWORD", "test-only-password")
	db, mock := bootstrapDB(t)
	mock.ExpectQuery("SELECT count").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .*tenants").WillReturnError(errors.New("storage unavailable"))
	mock.ExpectRollback()
	if err := SeedBootstrap(db); err == nil {
		t.Fatal("ignored storage failure")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func bootstrapDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	raw, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { raw.Close() })
	db, err := gorm.Open(postgres.New(postgres.Config{Conn: raw}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	return db, mock
}
