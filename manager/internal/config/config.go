package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

type NativeHostConfig struct {
	Address        string `mapstructure:"address"`
	User           string `mapstructure:"user"`
	PrivateKeyFile string `mapstructure:"private_key_file"`
	KnownHostsFile string `mapstructure:"known_hosts_file"`
}

type Config struct {
	ListenAddress string                      `mapstructure:"listen_address"`
	NativeHosts   map[string]NativeHostConfig `mapstructure:"native_hosts"`
	Port          int                         `mapstructure:"port"`
	LogLevel      string                      `mapstructure:"log_level"`
	Env           string                      `mapstructure:"env"`

	Database DatabaseConfig `mapstructure:"database"`
	Auth     AuthConfig     `mapstructure:"auth"`
	KubeVirt KubeVirtConfig `mapstructure:"kubevirt"`
	Ceph     CephConfig     `mapstructure:"ceph"`
	AI       AIConfig       `mapstructure:"ai"`
	CORS     CORSConfig     `mapstructure:"cors"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

func (d DatabaseConfig) DSN() string {
	u := url.URL{Scheme: "postgresql", User: url.UserPassword(d.User, d.Password), Path: "/" + d.DBName}
	q := url.Values{"sslmode": {d.SSLMode}}
	if strings.HasPrefix(d.Host, "/") {
		q.Set("host", d.Host)
		q.Set("port", strconv.Itoa(d.Port))
	} else {
		u.Host = net.JoinHostPort(d.Host, strconv.Itoa(d.Port))
	}
	u.RawQuery = q.Encode()
	return u.String()
}

type AuthConfig struct {
	JWTSecret        string `mapstructure:"jwt_secret"`
	TokenExpiry      int    `mapstructure:"token_expiry_hours"`
	OIDCIssuer       string `mapstructure:"oidc_issuer"`
	OIDCClientID     string `mapstructure:"oidc_client_id"`
	OIDCClientSecret string `mapstructure:"oidc_client_secret"`
	MFAEnabled       bool   `mapstructure:"mfa_enabled"`
	SessionTimeout   int    `mapstructure:"session_timeout_minutes"`
}

type KubeVirtConfig struct {
	Kubeconfig string `mapstructure:"kubeconfig"`
	Namespace  string `mapstructure:"namespace"`
	InCluster  bool   `mapstructure:"in_cluster"`
}

type CephConfig struct {
	MonHosts     []string `mapstructure:"mon_hosts"`
	AdminKeyring string   `mapstructure:"admin_keyring"`
	ClusterName  string   `mapstructure:"cluster_name"`
	DashboardURL string   `mapstructure:"dashboard_url"`
}

type AIConfig struct {
	Enabled           bool    `mapstructure:"enabled"`
	ModelEndpoint     string  `mapstructure:"model_endpoint"`
	AnomalyThreshold  float64 `mapstructure:"anomaly_threshold"`
	PredictionHorizon int     `mapstructure:"prediction_horizon_days"`
	AutoActions       bool    `mapstructure:"auto_actions"`
}

type CORSConfig struct {
	AllowedOrigins []string `mapstructure:"allowed_origins"`
	AllowedMethods []string `mapstructure:"allowed_methods"`
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/novasphere")
	viper.AddConfigPath("$HOME/.novasphere")

	// Defaults
	viper.SetDefault("port", 8080)
	viper.SetDefault("listen_address", "0.0.0.0")
	viper.SetDefault("log_level", "info")
	viper.SetDefault("env", "development")
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.user", "novasphere")
	viper.SetDefault("database.dbname", "novasphere")
	viper.SetDefault("database.sslmode", "disable")
	viper.SetDefault("auth.token_expiry_hours", 24)
	viper.SetDefault("auth.session_timeout_minutes", 30)
	viper.SetDefault("kubevirt.namespace", "default")
	viper.SetDefault("ceph.cluster_name", "ceph")
	viper.SetDefault("ai.enabled", true)
	viper.SetDefault("ai.anomaly_threshold", 0.85)
	viper.SetDefault("ai.prediction_horizon_days", 30)
	viper.SetDefault("cors.allowed_origins", []string{"http://localhost:3000"})
	viper.SetDefault("cors.allowed_methods", []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"})

	// Environment variable overrides
	viper.SetEnvPrefix("NOVA")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config: %w", err)
		}
		// Config file not found; rely on defaults and env vars
	}

	// Sensitive overrides after config file so compose env wins
	if v := os.Getenv("NOVA_DB_PASSWORD"); v != "" {
		viper.Set("database.password", v)
	}
	if v := os.Getenv("NOVA_DATABASE_PASSWORD"); v != "" {
		viper.Set("database.password", v)
	}
	if v := os.Getenv("NOVA_JWT_SECRET"); v != "" {
		viper.Set("auth.jwt_secret", v)
	}
	if v := os.Getenv("NOVA_AUTH_JWT_SECRET"); v != "" {
		viper.Set("auth.jwt_secret", v)
	}
	if v := os.Getenv("NOVA_DATABASE_HOST"); v != "" {
		viper.Set("database.host", v)
	}
	if v := os.Getenv("NOVA_DATABASE_USER"); v != "" {
		viper.Set("database.user", v)
	}
	if v := os.Getenv("NOVA_DATABASE_NAME"); v != "" {
		viper.Set("database.name", v)
		viper.Set("database.dbname", v)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Validate fails closed before any service starts with an absent signing key.
func (c Config) Validate() error {
	if len(c.Auth.JWTSecret) < 32 {
		return fmt.Errorf("auth.jwt_secret must contain at least 32 bytes of operator-supplied secret material")
	}
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	return nil
}
