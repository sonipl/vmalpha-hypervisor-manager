package handlers

import (
	"context"
	"io"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/novasphere/novasphere/internal/config"
	"github.com/sirupsen/logrus"
)

type endedConsoleStream struct{}

func (endedConsoleStream) Read([]byte) (int, error)    { return 0, io.EOF }
func (endedConsoleStream) Write(p []byte) (int, error) { return len(p), nil }
func (endedConsoleStream) Close() error                { return nil }

func TestConsoleStreamClosesWebSocketWhenHostStreamEnds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	host := config.NativeHostConfig{Address: "host.example:22", User: "manager"}
	h := &ConsoleHandler{
		hosts:  map[string]config.NativeHostConfig{"host": host},
		log:    logrus.New(),
		grants: map[string]consoleGrant{"one-use": {host: "host", guest: "fixture", expires: time.Now().Add(time.Minute)}},
		open: func(context.Context, config.NativeHostConfig, string) (io.ReadWriteCloser, error) {
			return endedConsoleStream{}, nil
		},
	}
	router := gin.New()
	router.GET("/stream", h.Stream)
	server := httptest.NewServer(router)
	defer server.Close()
	url := "ws" + strings.TrimPrefix(server.URL, "http") + "/stream?token=one-use"
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(time.Second))
	_, _, err = conn.ReadMessage()
	if err == nil {
		t.Fatal("expected the browser socket to close when the host stream reaches EOF")
	}
	if timeout, ok := err.(net.Error); ok && timeout.Timeout() {
		t.Fatalf("browser socket remained open after host EOF: %v", err)
	}
}
