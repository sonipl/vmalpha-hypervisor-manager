package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

const ldapSettingsPath = "/var/lib/novasphere/config/ldap.json"

// LDAPSettings stored for future auth integration (configure now, use later).
type LDAPSettings struct {
	Enabled    bool   `json:"enabled"`
	URL        string `json:"url"`
	BaseDN     string `json:"base_dn"`
	BindDN     string `json:"bind_dn"`
	BindSecret string `json:"bind_secret,omitempty"`
	UserFilter string `json:"user_filter"`
	GroupBase  string `json:"group_base"`
	Domain     string `json:"domain"`
	UseTLS     bool   `json:"use_tls"`
}

type SettingsHandler struct {
	DB  *gorm.DB
	Log *logrus.Logger
}

func NewSettingsHandler(db *gorm.DB, log *logrus.Logger) *SettingsHandler {
	return &SettingsHandler{DB: db, Log: log}
}

func loadLDAPSettings() LDAPSettings {
	def := LDAPSettings{
		Enabled:    false,
		URL:        "ldap://ldap01.vmalpha.com:389",
		BaseDN:     "dc=vmalpha,dc=com",
		BindDN:     "cn=Manager,dc=vmalpha,dc=com",
		UserFilter: "(uid=%s)",
		GroupBase:  "ou=groups,dc=vmalpha,dc=com",
		Domain:     "vmalpha.com",
	}
	b, err := os.ReadFile(ldapSettingsPath)
	if err != nil {
		return def
	}
	_ = json.Unmarshal(b, &def)
	return def
}

func saveLDAPSettings(s LDAPSettings) error {
	if err := os.MkdirAll(filepath.Dir(ldapSettingsPath), 0750); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ldapSettingsPath, b, 0600)
}

// GET /api/v1/settings/ldap
func (h *SettingsHandler) GetLDAP(c *gin.Context) {
	s := loadLDAPSettings()
	if s.BindSecret != "" {
		s.BindSecret = "********"
	}
	c.JSON(http.StatusOK, gin.H{"data": s})
}

// PUT /api/v1/settings/ldap
func (h *SettingsHandler) PutLDAP(c *gin.Context) {
	var req LDAPSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cur := loadLDAPSettings()
	if strings.TrimSpace(req.BindSecret) == "" || req.BindSecret == "********" {
		req.BindSecret = cur.BindSecret
	}
	if err := saveLDAPSettings(req); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := req
	out.BindSecret = "********"
	c.JSON(http.StatusOK, gin.H{"data": out, "message": "LDAP settings saved (auth integration pending)"})
}

// POST /api/v1/settings/ldap/test — stub connectivity check
func (h *SettingsHandler) TestLDAP(c *gin.Context) {
	s := loadLDAPSettings()
	if !s.Enabled {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "LDAP is disabled"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"ok":      true,
		"message": "LDAP test stub — configure bind credentials and enable when ldap01 is reachable",
		"url":     s.URL,
	})
}
