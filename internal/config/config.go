package config

import (
	"os"
)

type Config struct {
	Environment     string
	Port           string
	CookieSecret   string
	AWSAccessKey   string
	AWSSecretKey   string
	AWSRegion      string
	S3Bucket       string
	FromEmail      string
	TemplatesPath  string
	StaticPath     string
	UtilPath       string
	CloudPath      string
}

func Load() *Config {
	return &Config{
		Environment:     getEnv("ENVIRONMENT", "development"),
		Port:           getEnv("PORT", "8080"),
		CookieSecret:   getEnv("COOKIE_SECRET", "11oETzKXQAGaYdkL5gEmGeJJFuYh7EQnp2XdTP1o/Vo="),
		AWSAccessKey:   getEnv("AWS_ACCESS_KEY_ID", ""),
		AWSSecretKey:   getEnv("AWS_SECRET_ACCESS_KEY", ""),
		AWSRegion:      getEnv("AWS_REGION", "us-east-1"),
		S3Bucket:       getEnv("S3_BUCKET", "aspiring-cloud-storage"),
		FromEmail:      getEnv("FROM_EMAIL", "aspiring.investments@gmail.com"),
		TemplatesPath:  getEnv("TEMPLATES_PATH", "./web/templates"),
		StaticPath:     getEnv("STATIC_PATH", "./web/static"),
		UtilPath:       getEnv("UTIL_PATH", "./util"),
		CloudPath:      getEnv("CLOUD_PATH", "./cloud"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}