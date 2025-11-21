package config

import (
	"fmt"
)

// DatabaseConfig holds database configuration for both MongoDB and MySQL.
// It is derived from environment variables (or a .env file loaded via
// godotenv in main.go).
//
// Example environment variables:
//
//   STORAGE_BACKEND=mongodb
//
//   # MongoDB
//   MONGO_URI=mongodb://mongodb:27017
//   MONGO_DATABASE=touchcalc
//   # Optional, used to build MONGO_URI if not provided:
//   MONGO_HOST=mongodb
//   MONGO_PORT=27017
//
//   # MySQL
//   MYSQL_DSN=root:password@tcp(mysql:3306)/touchcalc?parseTime=true&charset=utf8mb4&loc=Local
//   # Optional, used to build MYSQL_DSN if not provided:
//   MYSQL_HOST=mysql
//   MYSQL_PORT=3306
//   MYSQL_USER=touchcalc
//   MYSQL_PASSWORD=touchcalc
//   MYSQL_DATABASE=touchcalc
//   MYSQL_PARAMS=parseTime=true&charset=utf8mb4&loc=Local
//
type DatabaseConfig struct {
	// Primary storage backend selector (e.g., "mongodb" or "mysql").
	StorageBackend string

	Mongo MongoDBConfig
	MySQL MySQLConfig
	SQLite SQLiteConfig
}

// MongoDBConfig holds MongoDB connection details.
type MongoDBConfig struct {
	URI      string
	Database string
	Host     string
	Port     string
}

// MySQLConfig holds MySQL connection details and a derived DSN.
type MySQLConfig struct {
	DSN      string
	Host     string
	Port     string
	User     string
	Password string
	Database string
	Params   string
}

// SQLiteConfig holds SQLite configuration.
type SQLiteConfig struct {
	DSN string
}

// LoadDatabaseConfig reads database configuration from environment
// variables, applies sensible defaults, and performs basic validation.
func LoadDatabaseConfig() (*DatabaseConfig, error) {
	cfg := &DatabaseConfig{
		StorageBackend: getEnv("STORAGE_BACKEND", "mongodb"),
	}

	// -------------------------------
	// MongoDB configuration
	// -------------------------------

	mongoURI := getEnv("MONGO_URI", "")
	mongoHost := getEnv("MONGO_HOST", "localhost")
	mongoPort := getEnv("MONGO_PORT", "27017")
	mongoDB := getEnv("MONGO_DATABASE", "touchcalc")

	if mongoURI == "" {
		mongoURI = fmt.Sprintf("mongodb://%s:%s", mongoHost, mongoPort)
	}

	cfg.Mongo = MongoDBConfig{
		URI:      mongoURI,
		Database: mongoDB,
		Host:     mongoHost,
		Port:     mongoPort,
	}

	// Basic validation: ensure URI and database are not empty
	if cfg.Mongo.URI == "" {
		return nil, fmt.Errorf("MONGO_URI (or MONGO_HOST/MONGO_PORT) must be set")
	}
	if cfg.Mongo.Database == "" {
		return nil, fmt.Errorf("MONGO_DATABASE must be set")
	}

	// -------------------------------
	// MySQL configuration
	// -------------------------------

	mysqlDSN := getEnv("MYSQL_DSN", "")
	mysqlHost := getEnv("MYSQL_HOST", "localhost")
	mysqlPort := getEnv("MYSQL_PORT", "3306")
	mysqlUser := getEnv("MYSQL_USER", "root")
	mysqlPassword := getEnv("MYSQL_PASSWORD", "password")
	mysqlDB := getEnv("MYSQL_DATABASE", "touchcalc")
	mysqlParams := getEnv("MYSQL_PARAMS", "parseTime=true&charset=utf8mb4&loc=Local")

	// If no explicit DSN is provided, build one from individual parts.
	if mysqlDSN == "" {
		if mysqlParams != "" {
			mysqlDSN = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?%s",
				mysqlUser, mysqlPassword, mysqlHost, mysqlPort, mysqlDB, mysqlParams)
		} else {
			mysqlDSN = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s",
				mysqlUser, mysqlPassword, mysqlHost, mysqlPort, mysqlDB)
		}
	}

	cfg.MySQL = MySQLConfig{
		DSN:      mysqlDSN,
		Host:     mysqlHost,
		Port:     mysqlPort,
		User:     mysqlUser,
		Password: mysqlPassword,
		Database: mysqlDB,
		Params:   mysqlParams,
	}

	// -------------------------------
	// SQLite configuration
	// -------------------------------

	sqliteDSN := getEnv("SQLITE_DSN", "file:touchcalc.db?_pragma=foreign_keys(1)")
	cfg.SQLite = SQLiteConfig{
		DSN: sqliteDSN,
	}

	return cfg, nil
}

// MongoURI builds the MongoDB connection string from the configuration.
// It simply returns the URI field, but is provided for symmetry with
// MySQLDSN and future extension.
func (c *DatabaseConfig) MongoURI() string {
	return c.Mongo.URI
}

// MySQLDSN builds the MySQL DSN from the configuration.
func (c *DatabaseConfig) MySQLDSN() string {
	return c.MySQL.DSN
}



