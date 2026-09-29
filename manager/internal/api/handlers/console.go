package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/novasphere/novasphere/internal/config"
	"github.com/novasphere/novasphere/internal/models"
	"github.com/novasphere/novasphere/internal/services/nativehost"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const consoleTokenLifetime = 60 * time.Second

type consoleGrant struct {
	host, guest string
	expires     time.Time
}

// ConsoleHandler brokers an authenticated Manager browser session through a
// trusted SSH host connection. Tokens are random, one-use, and intentionally
// short lived because browsers cannot add Bearer headers to a WebSocket upgrade.
type ConsoleHandler struct {
	db     *gorm.DB
	hosts  map[string]config.NativeHostConfig
	log    *logrus.Logger
	mu     sync.Mutex
	grants map[string]consoleGrant
}

func NewConsoleHandler(db *gorm.DB, hosts map[string]config.NativeHostConfig, log *logrus.Logger) *ConsoleHandler {
	return &ConsoleHandler{db: db, hosts: hosts, log: log, grants: make(map[string]consoleGrant)}
}

func (h *ConsoleHandler) Create(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid VM ID"})
		return
	}
	var vm models.VirtualMachine
	if err := h.db.First(&vm, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "VM not found"})
		return
	}
	if vm.Status != models.VMStatusRunning {
		c.JSON(http.StatusConflict, gin.H{"error": "VM must be running to open its console"})
		return
	}
	host, ok := h.resolveHost(vm.HostNode)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "the VM host is not enrolled for console access"})
		return
	}
	token, err := newConsoleToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "unable to create console session"})
		return
	}
	h.mu.Lock()
	for key, grant := range h.grants {
		if time.Now().After(grant.expires) {
			delete(h.grants, key)
		}
	}
	h.grants[token] = consoleGrant{host: host, guest: vm.Name, expires: time.Now().Add(consoleTokenLifetime)}
	h.mu.Unlock()
	c.JSON(http.StatusCreated, gin.H{"endpoint": "/api/v1/console/stream?token=" + token, "expires_in": int(consoleTokenLifetime.Seconds())})
}

func (h *ConsoleHandler) Stream(c *gin.Context) {
	grant, ok := h.consume(c.Query("token"))
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "console session is expired or invalid"})
		return
	}
	cfg, ok := h.hosts[grant.host]
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "console host is not enrolled"})
		return
	}
	client, err := nativehost.New(nativehost.Config{Address: cfg.Address, User: cfg.User, PrivateKeyFile: cfg.PrivateKeyFile, KnownHostsFile: cfg.KnownHostsFile})
	if err != nil {
		h.log.WithError(err).Warn("console host enrollment unavailable")
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "console host enrollment is unavailable"})
		return
	}
	stream, err := client.Console(context.Background(), grant.guest)
	if err != nil {
		h.log.WithError(err).Warn("console proxy unavailable")
		c.JSON(http.StatusBadGateway, gin.H{"error": "console proxy unavailable"})
		return
	}
	defer stream.Close()
	upgrader := websocket.Upgrader{CheckOrigin: sameConsoleOrigin}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 64*1024)
		for {
			n, readErr := stream.Read(buf)
			if n > 0 && conn.WriteMessage(websocket.BinaryMessage, buf[:n]) != nil {
				return
			}
			if readErr != nil {
				return
			}
		}
	}()
	for {
		kind, payload, readErr := conn.ReadMessage()
		if readErr != nil || kind == websocket.CloseMessage {
			return
		}
		if kind == websocket.BinaryMessage && len(payload) > 0 {
			if _, err := stream.Write(payload); err != nil {
				return
			}
		}
		select {
		case <-done:
			return
		default:
		}
	}
}

func sameConsoleOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true // non-browser clients cannot obtain a grant without Manager authentication
	}
	u, err := url.Parse(origin)
	return err == nil && u.Host == r.Host && (u.Scheme == "http" || u.Scheme == "https")
}

func (h *ConsoleHandler) consume(token string) (consoleGrant, bool) {
	if token == "" {
		return consoleGrant{}, false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	grant, ok := h.grants[token]
	delete(h.grants, token)
	return grant, ok && time.Now().Before(grant.expires)
}

func (h *ConsoleHandler) resolveHost(name string) (string, bool) {
	if _, ok := h.hosts[name]; ok {
		return name, true
	}
	short := strings.Split(strings.ToLower(name), ".")[0]
	for key := range h.hosts {
		if strings.Split(strings.ToLower(key), ".")[0] == short {
			return key, true
		}
	}
	return "", false
}

func newConsoleToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", errors.New("random source unavailable")
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
