package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/novasphere/novasphere/internal/models"
	"github.com/novasphere/novasphere/internal/services/nativehost"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"net/http"
	"regexp"
	"sort"
	"time"
)

type DistributedPortGroup struct {
	Name string `json:"name"`
	VLAN int    `json:"vlan_id"`
}
type DistributedSpec struct {
	Name       string                 `json:"name"`
	Bridge     string                 `json:"bridge"`
	Uplink     string                 `json:"uplink"`
	MTU        int                    `json:"mtu"`
	Hosts      []string               `json:"hosts"`
	PortGroups []DistributedPortGroup `json:"port_groups"`
}
type DistributedNetworkHandler struct {
	DB     *gorm.DB
	Native *NativeHandler
}

func NewDistributedNetworkHandler(db *gorm.DB, n *NativeHandler) *DistributedNetworkHandler {
	return &DistributedNetworkHandler{db, n}
}

var linuxInterfaceName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,14}$`)
var networkDisplayName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9 _.-]{0,62}$`)

func validateDistributedSpec(s DistributedSpec, n *NativeHandler) error {
	if !networkDisplayName.MatchString(s.Name) || !linuxInterfaceName.MatchString(s.Bridge) || !linuxInterfaceName.MatchString(s.Uplink) || s.Bridge == s.Uplink {
		return errors.New("valid name and distinct bridge/uplink interfaces are required")
	}
	if s.MTU < 1280 || s.MTU > 9000 {
		return errors.New("MTU must be between 1280 and 9000")
	}
	if n == nil || len(s.Hosts) == 0 || len(s.Hosts) > 64 {
		return errors.New("select enrolled target hosts")
	}
	seen := map[string]bool{}
	for _, id := range s.Hosts {
		if _, ok := n.Hosts[id]; !ok || seen[id] {
			return errors.New("target hosts must be unique enrolled hosts")
		}
		seen[id] = true
	}
	names := map[string]bool{}
	vlans := map[int]bool{}
	if len(s.PortGroups) > 64 {
		return errors.New("at most 64 port groups supported")
	}
	for _, p := range s.PortGroups {
		if !networkDisplayName.MatchString(p.Name) || p.VLAN < 0 || p.VLAN > 4094 || names[p.Name] || vlans[p.VLAN] {
			return errors.New("port groups need unique names and VLAN IDs from 0 through 4094")
		}
		names[p.Name] = true
		vlans[p.VLAN] = true
	}
	return nil
}
func (h *DistributedNetworkHandler) List(c *gin.Context) {
	var rows []models.DistributedNetwork
	if h.DB.Where("status <> ?", "deleted").Order("name").Find(&rows).Error != nil {
		c.JSON(500, gin.H{"error": "Network inventory unavailable"})
		return
	}
	c.JSON(200, gin.H{"data": rows})
}
func (h *DistributedNetworkHandler) Targets(c *gin.Context) {
	ids := []string{}
	for id := range h.Native.Hosts {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	c.JSON(200, ids)
}
func (h *DistributedNetworkHandler) call(ctx context.Context, host, operation string, args map[string]any) (json.RawMessage, error) {
	cfg, ok := h.Native.Hosts[host]
	if !ok {
		return nil, errors.New("host no longer enrolled")
	}
	b, e := h.Native.Connect(cfg)
	if e != nil {
		return nil, e
	}
	return b.Call(ctx, nativehost.Request{Operation: operation, Arguments: args})
}
func (h *DistributedNetworkHandler) Review(c *gin.Context) {
	var req struct {
		ID       string          `json:"id"`
		Action   string          `json:"action"`
		Revision uint            `json:"revision"`
		Spec     DistributedSpec `json:"spec"`
	}
	if c.ShouldBindJSON(&req) != nil {
		c.JSON(400, gin.H{"error": "Invalid review request"})
		return
	}
	if req.Action != "create" && req.Action != "update" && req.Action != "delete" {
		c.JSON(400, gin.H{"error": "Unsupported action"})
		return
	}
	if req.Action == "create" {
		req.ID = uuid.NewString()
		req.Revision = 0
	} else {
		var old models.DistributedNetwork
		if h.DB.First(&old, "id = ?", req.ID).Error != nil {
			c.JSON(404, gin.H{"error": "Network not found"})
			return
		}
		if old.Status == "deleted" || old.Status == "applying" || old.Status == "partial" || old.Revision != req.Revision {
			c.JSON(409, gin.H{"error": "Network revision changed or requires reconciliation"})
			return
		}
		var prev DistributedSpec
		if json.Unmarshal([]byte(old.Spec), &prev) != nil {
			c.JSON(409, gin.H{"error": "Stored configuration invalid"})
			return
		}
		if req.Action == "delete" {
			req.Spec = prev
		} else {
			// Moving a live uplink/bridge/target set is a migration, not a safe edit.
			a, b := append([]string{}, prev.Hosts...), append([]string{}, req.Spec.Hosts...)
			sort.Strings(a)
			sort.Strings(b)
			if prev.Bridge != req.Spec.Bridge || prev.Uplink != req.Spec.Uplink || fmt.Sprint(a) != fmt.Sprint(b) {
				c.JSON(409, gin.H{"error": "Bridge, uplink and host membership require a separately planned migration"})
				return
			}
		}
	}
	if err := validateDistributedSpec(req.Spec, h.Native); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	previews := map[string]json.RawMessage{}
	failures := map[string]string{}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	for _, host := range req.Spec.Hosts {
		raw, err := h.call(ctx, host, "network.distributed.preview", map[string]any{"id": req.ID, "action": req.Action, "spec": req.Spec})
		var result struct {
			Supported   bool   `json:"supported"`
			Fingerprint string `json:"fingerprint"`
		}
		if err != nil || json.Unmarshal(raw, &result) != nil || !result.Supported || result.Fingerprint == "" {
			failures[host] = "Host preflight unavailable or refused; no changes applied"
			continue
		}
		previews[host] = raw
	}
	if len(failures) > 0 {
		c.JSON(409, gin.H{"error": "Every target must pass preflight before applying", "hosts": failures, "previews": previews})
		return
	}
	spec, _ := json.Marshal(req.Spec)
	raw, _ := json.Marshal(previews)
	owner, _ := c.Get("user_id")
	review := models.DistributedNetworkReview{ID: uuid.NewString(), NetworkID: req.ID, Owner: fmt.Sprint(owner), Action: req.Action, Revision: req.Revision, Spec: string(spec), Previews: string(raw), State: "reviewed", ExpiresAt: time.Now().UTC().Add(5 * time.Minute)}
	if h.DB.Create(&review).Error != nil {
		c.JSON(500, gin.H{"error": "Cannot persist review"})
		return
	}
	c.JSON(200, review)
}
func (h *DistributedNetworkHandler) Apply(c *gin.Context) {
	var req struct {
		ReviewID string `json:"review_id"`
		Confirm  bool   `json:"confirm"`
	}
	if c.ShouldBindJSON(&req) != nil || !req.Confirm {
		c.JSON(400, gin.H{"error": "Explicit confirmation of a review is required"})
		return
	}
	owner, _ := c.Get("user_id")
	var review models.DistributedNetworkReview
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&review, "id = ?", req.ReviewID).Error; err != nil {
			return err
		}
		if review.Owner != fmt.Sprint(owner) || review.State != "reviewed" || time.Now().After(review.ExpiresAt) {
			return errors.New("review expired, already used, or belongs to another user")
		}
		var spec DistributedSpec
		if json.Unmarshal([]byte(review.Spec), &spec) != nil {
			return errors.New("invalid review")
		}
		if review.Action == "create" {
			if err := tx.Create(&models.DistributedNetwork{ID: review.NetworkID, Name: spec.Name, Spec: review.Spec, Revision: 1, Status: "applying", Results: "{}"}).Error; err != nil {
				return err
			}
		} else {
			result := tx.Model(&models.DistributedNetwork{}).Where("id = ? AND revision = ? AND status NOT IN ?", review.NetworkID, review.Revision, []string{"applying", "partial"}).Updates(map[string]any{"status": "applying", "revision": review.Revision + 1, "spec": review.Spec, "name": spec.Name})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errors.New("network changed after review")
			}
		}
		return tx.Model(&review).Update("state", "applying").Error
	})
	if err != nil {
		c.JSON(409, gin.H{"error": "Review cannot be applied; refresh and review again"})
		return
	}
	// Keep the request bounded; persisted applying state survives a Manager crash
	// and must be reconciled explicitly, never replayed automatically.
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var spec DistributedSpec
	_ = json.Unmarshal([]byte(review.Spec), &spec)
	var previews map[string]struct {
		Fingerprint string `json:"fingerprint"`
	}
	_ = json.Unmarshal([]byte(review.Previews), &previews)
	results := map[string]any{}
	failed := false
	for _, host := range spec.Hosts {
		if failed {
			results[host] = gin.H{"state": "not_attempted"}
			continue
		}
		raw, err := h.call(ctx, host, "network.distributed.apply", map[string]any{"id": review.NetworkID, "action": review.Action, "spec": spec, "expected_fingerprint": previews[host].Fingerprint})
		var result struct {
			Applied bool `json:"applied"`
		}
		if err != nil || json.Unmarshal(raw, &result) != nil || !result.Applied {
			failed = true
			results[host] = gin.H{"state": "failed", "message": "Host refused or failed; inspect host before retry"}
		} else {
			results[host] = gin.H{"state": "applied", "result": json.RawMessage(raw)}
		}
	}
	status := "applied"
	if failed {
		status = "partial"
	} else if review.Action == "delete" {
		status = "deleted"
	}
	raw, _ := json.Marshal(results)
	updates := map[string]any{"status": status, "results": string(raw)}
	if status == "deleted" {
		updates["name"] = review.NetworkID
	}
	if err := h.DB.Model(&models.DistributedNetwork{}).Where("id = ?", review.NetworkID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Host execution finished but result persistence failed; reconcile before retry"})
		return
	}

	h.DB.Model(&review).Update("state", status)
	c.JSON(200, gin.H{"id": review.NetworkID, "status": status, "hosts": results})
}
