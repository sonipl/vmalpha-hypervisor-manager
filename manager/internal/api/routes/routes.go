package routes

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/novasphere/novasphere/internal/api/handlers"
	"github.com/novasphere/novasphere/internal/api/middleware"
	"github.com/novasphere/novasphere/internal/config"
	ws "github.com/novasphere/novasphere/internal/websocket"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func Setup(cfg *config.Config, db *gorm.DB, hub *ws.Hub, log *logrus.Logger) *gin.Engine {
	return setupWithMiddleware(cfg, db, hub, log, middleware.AuthMiddleware(cfg.Auth.JWTSecret, db), middleware.AuditMiddleware(db, log))
}

func setupWithMiddleware(cfg *config.Config, db *gorm.DB, hub *ws.Hub, log *logrus.Logger, authenticate, audit gin.HandlerFunc) *gin.Engine {
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.CORSMiddleware(cfg.CORS.AllowedOrigins))

	// Initialize handlers
	vmHandler := handlers.NewVMHandler(db, hub, log)
	hostHandler := handlers.NewHostHandler(db, hub, log)
	nativeHandler := handlers.NewNativeHandler(db, cfg.NativeHosts)
	hostHandler.Native = nativeHandler
	storageHandler := handlers.NewStorageHandler(db, log)
	storageBackendsHandler := handlers.NewStorageBackendsHandler(db, log)
	storageBackendsHandler.Native = nativeHandler
	storageBackendsHandler.StartNativeRegistrationRefresh("kvm11")
	networkHandler := handlers.NewNetworkHandler(db, log)
	distributedNetworkHandler := handlers.NewDistributedNetworkHandler(db, nativeHandler)
	authHandler := handlers.NewAuthHandler(db, log, cfg.Auth.JWTSecret, cfg.Auth.TokenExpiry)
	aiHandler := handlers.NewAIHandler(db, log)
	monitoringHandler := handlers.NewMonitoringHandler(db, log)
	monitoringHandler.Native = nativeHandler
	setupHandler := handlers.NewSetupHandler(db, log)
	settingsHandler := handlers.NewSettingsHandler(db, log)
	cephMetricsHandler := handlers.NewCephMetricsHandlerWithNative(db, log, nativeHandler)
	monitoringHandler.ClusterHealth = cephMetricsHandler.HealthStatus

	// ── Public routes ──
	r.POST("/api/v1/auth/login", authHandler.Login)

	// Health check
	r.GET("/api/v1/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "version": "1.0.0"})
	})

	// ── Authenticated routes ──
	auth := r.Group("/api/v1")
	auth.Use(authenticate)
	auth.Use(audit)
	{
		// Profile
		auth.GET("/auth/profile", authHandler.GetProfile)

		// WebSocket
		auth.GET("/ws", hub.HandleWebSocket)

		// ── Virtual Machines ──
		vms := auth.Group("/vms")
		{
			vms.GET("", middleware.RBACMiddleware("virtualmachines", "list"), vmHandler.ListVMs)
			vms.GET("/:id", middleware.RBACMiddleware("virtualmachines", "read"), vmHandler.GetVM)
			vms.POST("", middleware.RBACMiddleware("virtualmachines", "create"), vmHandler.CreateVM)
			vms.PUT("/:id", middleware.RBACMiddleware("virtualmachines", "update"), vmHandler.UpdateVM)
			vms.DELETE("/:id", middleware.RBACMiddleware("virtualmachines", "delete"), vmHandler.DeleteVM)
			vms.POST("/:id/actions/:action", middleware.RBACMiddleware("virtualmachines", "update"), vmHandler.VMAction)
		}

		// Native enrollment and operations currently require a live Platform Admin.
		native := auth.Group("/native")
		native.Use(middleware.RBACMiddleware("native-hosts", "admin"))
		native.GET("/hosts", nativeHandler.List)
		native.GET("/hosts/:host/inventory", nativeHandler.Inventory)
		native.GET("/hosts/:host/metrics", nativeHandler.Metrics)
		native.GET("/hosts/:host/files", nativeHandler.Files)
		native.POST("/hosts/:host/operations", nativeHandler.Operate)
		native.GET("/tasks/:id", nativeHandler.Task)

		// ── Compute / Hosts ──
		hosts := auth.Group("/hosts")
		{
			hosts.GET("", middleware.RBACMiddleware("hosts", "list"), hostHandler.ListHosts)
			hosts.GET("/:id", middleware.RBACMiddleware("hosts", "read"), hostHandler.GetHost)
			hosts.POST("/:id/maintenance", middleware.RBACMiddleware("hosts", "update"), hostHandler.MaintenanceMode)
		}

		clusters := auth.Group("/clusters")
		{
			clusters.GET("", middleware.RBACMiddleware("clusters", "list"), hostHandler.ListClusters)
			clusters.GET("/:id", middleware.RBACMiddleware("clusters", "read"), hostHandler.GetCluster)
			clusters.POST("", middleware.RBACMiddleware("clusters", "create"), hostHandler.CreateCluster)
		}

		// ── Storage ──
		storage := auth.Group("/storage")
		{
			storage.GET("/templates", middleware.RBACMiddleware("templates", "list"), storageBackendsHandler.ListTemplates)
			storage.POST("/templates/import", middleware.RBACMiddleware("storage", "create"), storageBackendsHandler.RegisterDatastoreTemplate)
			storage.GET("/ceph/health", middleware.RBACMiddleware("storage", "list"), cephMetricsHandler.GetHealth)
			storage.GET("/ceph/metrics", middleware.RBACMiddleware("storage", "list"), cephMetricsHandler.GetMetrics)
			storage.GET("/classes", middleware.RBACMiddleware("storage", "list"), storageBackendsHandler.ListClasses)
			storage.POST("/local", middleware.RBACMiddleware("storage", "create"), storageBackendsHandler.AddLocal)
			storage.POST("/nfs", middleware.RBACMiddleware("storage", "create"), storageBackendsHandler.AddNFS)
			storage.POST("/external-nfs", middleware.RBACMiddleware("storage", "create"), storageBackendsHandler.AddExternalNFS)
			storage.GET("/backends/:id/browser", middleware.RBACMiddleware("storage", "list"), storageBackendsHandler.BrowseDatastore)
			storage.GET("/backends/:id/browser/download", middleware.RBACMiddleware("storage", "list"), storageBackendsHandler.DownloadDatastore)
			storage.POST("/backends/:id/browser/folder", middleware.RBACMiddleware("storage", "create"), storageBackendsHandler.MkdirDatastore)
			storage.POST("/backends/:id/browser/upload", middleware.RBACMiddleware("storage", "create"), storageBackendsHandler.UploadDatastore)
			storage.DELETE("/backends/:id/browser/entry", middleware.RBACMiddleware("storage", "delete"), storageBackendsHandler.DeleteDatastoreEntry)
			storage.POST("/ceph", middleware.RBACMiddleware("storage", "create"), storageBackendsHandler.AddCeph)
			storage.GET("/backends/:id/status", middleware.RBACMiddleware("storage", "list"), storageBackendsHandler.BackendStatus)
			storage.POST("/backends/:id/mount", middleware.RBACMiddleware("storage", "update"), storageBackendsHandler.MountBackend)
			storage.POST("/backends/:id/unmount", middleware.RBACMiddleware("storage", "update"), storageBackendsHandler.UnmountBackend)
			storage.DELETE("/backends/:id", middleware.RBACMiddleware("storage", "delete"), storageBackendsHandler.DeleteBackend)
			storage.POST("/classes", middleware.RBACMiddleware("storage", "create"), storageHandler.CreateStorageClass)
			storage.PUT("/classes/:id", middleware.RBACMiddleware("storage", "update"), storageHandler.UpdateStorageClass)

			storage.GET("/volumes", middleware.RBACMiddleware("volumes", "list"), storageHandler.ListVolumes)
			storage.POST("/volumes", middleware.RBACMiddleware("volumes", "create"), storageHandler.CreateVolume)
			storage.POST("/volumes/:id/expand", middleware.RBACMiddleware("volumes", "update"), storageHandler.ExpandVolume)
			storage.DELETE("/volumes/:id", middleware.RBACMiddleware("volumes", "delete"), storageHandler.DeleteVolume)

			storage.GET("/snapshots", middleware.RBACMiddleware("snapshots", "list"), storageHandler.ListSnapshots)
			storage.POST("/snapshots/:id/restore", middleware.RBACMiddleware("snapshots", "update"), storageHandler.RestoreSnapshot)
			storage.DELETE("/snapshots/:id", middleware.RBACMiddleware("snapshots", "delete"), storageHandler.DeleteSnapshot)

			storage.GET("/backups", middleware.RBACMiddleware("backups", "list"), storageHandler.ListBackupJobs)
			storage.POST("/backups", middleware.RBACMiddleware("backups", "create"), storageHandler.CreateBackupJob)
		}

		// Explicit, reviewed native-network operations only; reads never fan out mutations.
		distributed := auth.Group("/distributed-networks")
		distributed.GET("", middleware.RBACMiddleware("networks", "list"), distributedNetworkHandler.List)
		distributed.GET("/targets", middleware.RBACMiddleware("networks", "list"), distributedNetworkHandler.Targets)
		distributed.POST("/discover", middleware.RBACMiddleware("networks", "list"), distributedNetworkHandler.Discover)
		distributed.POST("/review", middleware.RBACMiddleware("networks", "create"), middleware.RBACMiddleware("networks", "update"), middleware.RBACMiddleware("networks", "delete"), distributedNetworkHandler.Review)
		distributed.POST("/apply", middleware.RBACMiddleware("networks", "create"), middleware.RBACMiddleware("networks", "update"), middleware.RBACMiddleware("networks", "delete"), distributedNetworkHandler.Apply)
		// ── Networking ──
		networks := auth.Group("/networks")
		{
			networks.GET("", middleware.RBACMiddleware("networks", "list"), networkHandler.ListNetworks)
			networks.GET("/:id", middleware.RBACMiddleware("networks", "read"), networkHandler.GetNetwork)
			networks.POST("", middleware.RBACMiddleware("networks", "create"), networkHandler.CreateNetwork)
			networks.PUT("/:id", middleware.RBACMiddleware("networks", "update"), networkHandler.UpdateNetwork)
			networks.DELETE("/:id", middleware.RBACMiddleware("networks", "delete"), networkHandler.DeleteNetwork)
		}

		firewalls := auth.Group("/firewalls")
		{
			firewalls.GET("", middleware.RBACMiddleware("firewalls", "list"), networkHandler.ListFirewallRules)
			firewalls.POST("", middleware.RBACMiddleware("firewalls", "create"), networkHandler.CreateFirewallRule)
			firewalls.DELETE("/:id", middleware.RBACMiddleware("firewalls", "delete"), networkHandler.DeleteFirewallRule)
		}

		// ── Security / Auth Management ──
		users := auth.Group("/users")
		{
			users.GET("", middleware.RBACMiddleware("users", "list"), authHandler.ListUsers)
			users.POST("", middleware.RBACMiddleware("users", "create"), authHandler.CreateUser)
		}

		roles := auth.Group("/roles")
		{
			roles.GET("", middleware.RBACMiddleware("roles", "list"), authHandler.ListRoles)
			roles.POST("", middleware.RBACMiddleware("roles", "create"), authHandler.CreateRole)
		}

		tenants := auth.Group("/tenants")
		{
			tenants.GET("", middleware.RBACMiddleware("users", "list"), authHandler.ListTenants)
			tenants.POST("", middleware.RBACMiddleware("users", "create"), authHandler.CreateTenant)
		}

		auth.GET("/audit", middleware.RBACMiddleware("audit", "list"), authHandler.ListAuditLogs)

		// ── AI / NovaMind ──
		ai := auth.Group("/ai")
		{
			ai.GET("/recommendations", middleware.RBACMiddleware("ai", "read"), aiHandler.ListRecommendations)
			ai.POST("/recommendations/:id/apply", middleware.RBACMiddleware("ai", "create"), aiHandler.ApplyRecommendation)
			ai.POST("/recommendations/:id/dismiss", middleware.RBACMiddleware("ai", "create"), aiHandler.DismissRecommendation)
			ai.GET("/insights", middleware.RBACMiddleware("ai", "read"), aiHandler.GetInsights)
			ai.POST("/chat", middleware.RBACMiddleware("ai", "create"), aiHandler.Chat)
			ai.POST("/migration/assess", middleware.RBACMiddleware("ai", "create"), aiHandler.AssessMigration)

			ai.GET("/policies", middleware.RBACMiddleware("ai", "read"), aiHandler.ListPolicies)
			ai.POST("/policies", middleware.RBACMiddleware("ai", "create"), aiHandler.CreatePolicy)
			ai.PUT("/policies/:id", middleware.RBACMiddleware("ai", "update"), aiHandler.UpdatePolicy)
		}

		// ── Setup (day-2 Ceph / KubeVirt nodes) ──
		setup := auth.Group("/setup")
		{
			setup.GET("/status", middleware.RBACMiddleware("setup", "read"), setupHandler.GetStatus)
			setup.POST("/nodes", middleware.RBACMiddleware("setup", "create"), setupHandler.AddNode)
			setup.POST("/nodes/:id/join-k8s", middleware.RBACMiddleware("setup", "create"), setupHandler.JoinK8s)
			setup.POST("/ceph/hosts", middleware.RBACMiddleware("setup", "create"), setupHandler.AddCephHost)
			setup.POST("/ceph/enable", middleware.RBACMiddleware("setup", "create"), setupHandler.EnableCeph)
			setup.DELETE("/nodes/:id", middleware.RBACMiddleware("setup", "delete"), setupHandler.RemoveNode)
		}

		// ── Settings ──
		settings := auth.Group("/settings")
		{
			settings.GET("/ldap", middleware.RBACMiddleware("settings", "read"), settingsHandler.GetLDAP)
			settings.PUT("/ldap", middleware.RBACMiddleware("settings", "update"), settingsHandler.PutLDAP)
			settings.POST("/ldap/test", middleware.RBACMiddleware("settings", "create"), settingsHandler.TestLDAP)
		}

		// ── Monitoring ──
		monitoring := auth.Group("/monitoring")
		{
			monitoring.GET("/dashboard", middleware.RBACMiddleware("metrics", "read"), monitoringHandler.GetDashboardMetrics)
			monitoring.GET("/events", middleware.RBACMiddleware("metrics", "read"), monitoringHandler.GetEvents)

			monitoring.GET("/alerts", middleware.RBACMiddleware("metrics", "read"), monitoringHandler.ListAlertRules)
			monitoring.POST("/alerts", middleware.RBACMiddleware("alerts", "create"), monitoringHandler.CreateAlertRule)
			monitoring.PUT("/alerts/:id", middleware.RBACMiddleware("alerts", "update"), monitoringHandler.UpdateAlertRule)
			monitoring.DELETE("/alerts/:id", middleware.RBACMiddleware("alerts", "delete"), monitoringHandler.DeleteAlertRule)
		}
	}

	// ── SPA / static UI (built into image at web/dist) ──
	r.Static("/assets", "./web/dist/assets")
	r.StaticFile("/favicon.ico", "./web/dist/favicon.ico")
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.File("./web/dist/index.html")
	})

	return r
}
