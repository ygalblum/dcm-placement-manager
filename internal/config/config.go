package config

import (
	"log"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Service  *ServiceConfig
	Database *DBConfig
}

type ServiceConfig struct {
	Address  string `envconfig:"SVC_ADDRESS" default:":8080"`
	LogLevel string `envconfig:"SVC_LOG_LEVEL" default:"info"`
}

// DBConfig holds database configuration
type DBConfig struct {
	Type     string `envconfig:"DB_TYPE" default:"pgsql"`
	Hostname string `envconfig:"DB_HOST" default:"localhost"`
	Port     string `envconfig:"DB_PORT" default:"5432"`
	Name     string `envconfig:"DB_NAME" default:"placement-manager"`
	User     string `envconfig:"DB_USER"`
	Password string `envconfig:"DB_PASSWORD"`
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	cfg := &Config{}
	if err := envconfig.Process("", cfg); err != nil {
		return nil, err
	}

	// Validate and set defaults for Database.Type
	if cfg.Database.Type != "pgsql" && cfg.Database.Type != "sqlite" {
		log.Printf("WARNING: invalid DB_TYPE %q, defaulting to sqlite", cfg.Database.Type)
		cfg.Database.Type = "sqlite"
	}

	return cfg, nil
}
