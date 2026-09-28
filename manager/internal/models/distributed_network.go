package models

import "time"

// DistributedNetwork retains desired configuration and per-host apply outcomes.
// A partial result is retained for review rather than hidden by database deletion.
type DistributedNetwork struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"uniqueIndex"`
	Spec      string    `json:"spec" gorm:"type:text"`
	Revision  uint      `json:"revision"`
	Status    string    `json:"status"`
	Results   string    `json:"results" gorm:"type:text"`
	UpdatedAt time.Time `json:"updated_at"`
}
type DistributedNetworkReview struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	NetworkID string    `json:"network_id" gorm:"index"`
	Owner     string    `json:"owner"`
	Action    string    `json:"action"`
	Revision  uint      `json:"revision"`
	Spec      string    `json:"spec" gorm:"type:text"`
	Previews  string    `json:"previews" gorm:"type:text"`
	State     string    `json:"state"`
	ExpiresAt time.Time `json:"expires_at"`
}
