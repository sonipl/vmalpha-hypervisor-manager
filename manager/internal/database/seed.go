package database

import (
	"fmt"
	"os"

	"github.com/novasphere/novasphere/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// SeedBootstrap creates the first administrator only with an operator-supplied
// password. Existing installations never have their credentials reset.
func SeedBootstrap(db *gorm.DB) error {
	var count int64
	if err := db.Model(&models.User{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	password := os.Getenv("NOVA_BOOTSTRAP_PASSWORD")
	if len(password) < 12 || len(password) > 72 {
		return fmt.Errorf("first startup requires NOVA_BOOTSTRAP_PASSWORD containing 12–72 bytes")
	}
	return db.Transaction(func(db *gorm.DB) error {
		tenant := models.Tenant{
			Name:           "default",
			Description:    "Default tenant",
			MaxVCPUs:       256,
			MaxMemoryMB:    1048576,
			MaxStorageGB:   10240,
			MaxVMs:         500,
			MaxSnapshots:   1000,
			CustomBranding: models.JSONMap{},
		}
		if err := db.Where("name = ?", tenant.Name).FirstOrCreate(&tenant).Error; err != nil {
			return fmt.Errorf("seed tenant: %w", err)
		}

		role := models.Role{
			Name:        "Platform Admin",
			Description: "Full platform administrator",
			Scope:       "Global",
			IsBuiltIn:   true,
			Permissions: models.JSONMap{},
		}
		if err := db.Where("name = ?", role.Name).FirstOrCreate(&role).Error; err != nil {
			return fmt.Errorf("seed role: %w", err)
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash password: %w", err)
		}

		user := models.User{
			Username: "admin",
			Email:    "admin@vmalpha.com",
			FullName: "VM Alpha Administrator",
			Password: string(hash),
			RoleID:   role.ID,
			TenantID: tenant.ID,
			IsActive: true,
			Source:   "local",
		}
		if err := db.Where("username = ?", user.Username).FirstOrCreate(&user).Error; err != nil {
			return fmt.Errorf("seed user: %w", err)
		}
		return nil
	})
}
