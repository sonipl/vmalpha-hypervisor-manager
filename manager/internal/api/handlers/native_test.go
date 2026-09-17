package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/novasphere/novasphere/internal/config"
	"github.com/novasphere/novasphere/internal/services/nativehost"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeNativeBroker struct {
	calls int
	err   error
}

func (f *fakeNativeBroker) Call(context.Context, nativehost.Request) (json.RawMessage, error) {
	f.calls++
	return json.RawMessage(`"done"`), f.err
}
func nativeTestHandler(t *testing.T) (*NativeHandler, sqlmock.Sqlmock, *fakeNativeBroker) {
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
	broker := &fakeNativeBroker{}
	h := NewNativeHandler(db, map[string]config.NativeHostConfig{"lab": {}})
	h.Connect = func(config.NativeHostConfig) (nativeBroker, error) { return broker, nil }
	return h, mock, broker
}
func invokeNative(h *NativeHandler, actor, id uuid.UUID, body string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", actor) })
	r.POST("/:host", h.Operate)
	req := httptest.NewRequest("POST", "/lab", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", id.String())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

const nativeBody = `{"op":"vm-action","args":{"name":"fixture","action":"start"}}`

func TestNativeStoreFailureDoesNotMutateHost(t *testing.T) {
	h, mock, broker := nativeTestHandler(t)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO .*native_tasks").WillReturnError(errors.New("offline"))
	mock.ExpectRollback()
	w := invokeNative(h, uuid.New(), uuid.New(), nativeBody)
	if w.Code != 503 || broker.calls != 0 {
		t.Fatalf("status %d calls %d: %s", w.Code, broker.calls, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestNativeMutationRecordedBeforeCall(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "uncertain"}[fail], func(t *testing.T) {
			h, mock, broker := nativeTestHandler(t)
			if fail {
				broker.err = errors.New("disconnect")
			}
			mock.ExpectBegin()
			mock.ExpectExec("INSERT INTO .*native_tasks").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			mock.ExpectBegin()
			mock.ExpectExec("UPDATE .*native_tasks").WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
			w := invokeNative(h, uuid.New(), uuid.New(), nativeBody)
			want := 200
			status := "Succeeded"
			if fail {
				want = 502
				status = "Unknown"
			}
			if w.Code != want || broker.calls != 1 || !strings.Contains(w.Body.String(), status) {
				t.Fatalf("status %d calls %d: %s", w.Code, broker.calls, w.Body.String())
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestNativeDuplicateDoesNotRepeatMutation(t *testing.T) {
	h, mock, broker := nativeTestHandler(t)
	actor, id := uuid.New(), uuid.New()
	var req nativehost.Request
	_ = json.Unmarshal([]byte(nativeBody), &req)
	raw, _ := json.Marshal(req)
	hash := sha256.Sum256(raw)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO .*native_tasks").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT .*native_tasks").WillReturnRows(sqlmock.NewRows([]string{"id", "actor_id", "host", "request_hash", "status"}).AddRow(id, actor, "lab", hex.EncodeToString(hash[:]), "Running"))
	w := invokeNative(h, actor, id, nativeBody)
	if w.Code != 200 || broker.calls != 0 || !strings.Contains(w.Body.String(), `"replayed":true`) {
		t.Fatalf("status %d calls %d: %s", w.Code, broker.calls, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestExplicitBrokerFailureIsRecordedAsFailed(t *testing.T) {
	h, mock, broker := nativeTestHandler(t)
	broker.err = &nativehost.BrokerError{}
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO .*native_tasks").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE .*native_tasks").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	w := invokeNative(h, uuid.New(), uuid.New(), nativeBody)
	if w.Code != 502 || broker.calls != 1 || !strings.Contains(w.Body.String(), `"status":"Failed"`) {
		t.Fatalf("status %d calls %d: %s", w.Code, broker.calls, w.Body.String())
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
