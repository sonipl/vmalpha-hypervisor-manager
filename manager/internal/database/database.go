package database

import (
	"github.com/novasphere/novasphere/internal/config"
	"github.com/novasphere/novasphere/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"log"
	"os"
	"time"
)

func Connect(cfg config.DatabaseConfig) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.New(log.New(os.Stdout, "", log.LstdFlags), logger.Config{
			LogLevel: logger.Warn, SlowThreshold: time.Second, ParameterizedQueries: true,
		}),
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	return db, nil
}

func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.NativeTask{},
		&models.Tenant{},
		&models.Role{},
		&models.User{},
		&models.Cluster{},
		&models.Host{},
		&models.StorageClass{},
		&models.Network{},
		&models.DistributedNetwork{},
		&models.DistributedNetworkReview{},
		&models.VMTemplate{},
		&models.VirtualMachine{},
		&models.VMDisk{},
		&models.VMNIC{},
		&models.VMSnapshot{},
		&models.Volume{},
		&models.FirewallRule{},
		&models.AuditLog{},
		&models.AIRecommendation{},
		&models.AutomationPolicy{},
		&models.MigrationPlan{},
		&models.MigrationPlanVM{},
		&models.AlertRule{},
		&models.BackupJob{},
		&models.CephMetricSample{},
	)
}
