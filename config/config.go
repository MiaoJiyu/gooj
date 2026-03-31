package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// DatabaseType represents the type of database
type DatabaseType string

const (
	DatabaseTypeSQLite DatabaseType = "sqlite"
	DatabaseTypeMySQL  DatabaseType = "mysql"
)

// Config holds all configuration for the application
type Config struct {
	Database DatabaseConfig `yaml:"database"`
	Server   ServerConfig   `yaml:"server"`
	Cmd      CmdConfig      `yaml:"cmd"`
	Services ServicesConfig `yaml:"services"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Type   DatabaseType `yaml:"type"`
	SQLite SQLiteConfig `yaml:"sqlite"`
	MySQL  MySQLConfig  `yaml:"mysql"`
}

// SQLiteConfig holds SQLite-specific configuration
type SQLiteConfig struct {
	Path string `yaml:"path"`
}

// MySQLConfig holds MySQL-specific configuration
type MySQLConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port int `yaml:"port"`
}

// CmdConfig holds command service configuration
type CmdConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// ServicesConfig holds service enable/disable flags
type ServicesConfig struct {
	SQL   bool `yaml:"sql"`
	Judge bool `yaml:"judge"`
	File  bool `yaml:"file"`
}

// GlobalConfig is the global configuration instance
var GlobalConfig *Config

// Load reads and parses the configuration file
func Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	GlobalConfig = &cfg
	return nil
}

// GetDatabaseType returns the database type from config
func GetDatabaseType() DatabaseType {
	if GlobalConfig == nil {
		return DatabaseTypeSQLite
	}
	return GlobalConfig.Database.Type
}

// GetSQLitePath returns the SQLite database path from config
func GetSQLitePath() string {
	if GlobalConfig == nil || GlobalConfig.Database.SQLite.Path == "" {
		return "data/app.db"
	}
	return GlobalConfig.Database.SQLite.Path
}

// GetMySQLConfig returns the MySQL configuration from config
func GetMySQLConfig() MySQLConfig {
	if GlobalConfig == nil {
		return MySQLConfig{
			Host:     "localhost",
			Port:     3306,
			User:     "root",
			Password: "",
			DBName:   "gooj",
		}
	}
	return GlobalConfig.Database.MySQL
}

// GetServerPort returns the server port from config
func GetServerPort() int {
	if GlobalConfig == nil {
		return 8081
	}
	return GlobalConfig.Server.Port
}

// GetCmdHost returns the command service host from config
func GetCmdHost() string {
	if GlobalConfig == nil {
		return "127.0.0.1"
	}
	return GlobalConfig.Cmd.Host
}

// GetCmdPort returns the command service port from config
func GetCmdPort() int {
	if GlobalConfig == nil {
		return 9090
	}
	return GlobalConfig.Cmd.Port
}

// GetCmdAddr returns the full command service address
func GetCmdAddr() string {
	return fmt.Sprintf("%s:%d", GetCmdHost(), GetCmdPort())
}

// GetServiceEnabled returns whether a specific service is enabled
func GetServiceEnabled(service string) bool {
	if GlobalConfig == nil {
		return true // default to enabled
	}

	switch service {
	case "sql":
		return GlobalConfig.Services.SQL
	case "judge":
		return GlobalConfig.Services.Judge
	case "file":
		return GlobalConfig.Services.File
	default:
		return true
	}
}

// IsSQLEnabled returns whether SQL service is enabled
func IsSQLEnabled() bool {
	return GetServiceEnabled("sql")
}

// IsJudgeEnabled returns whether judge service is enabled
func IsJudgeEnabled() bool {
	return GetServiceEnabled("judge")
}

// IsFileEnabled returns whether file service is enabled
func IsFileEnabled() bool {
	return GetServiceEnabled("file")
}
