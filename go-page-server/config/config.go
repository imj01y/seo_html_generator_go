// Package config handles configuration loading from YAML files
package config

import (
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration
type Config struct {
	Server         ServerConfig         `yaml:"server"`
	Database       DatabaseConfig       `yaml:"database"`
	Cache          CacheConfig          `yaml:"cache"`
	SpiderDetector SpiderDetectorConfig `yaml:"spider_detector"`
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
	Workers int    `yaml:"workers"`
	Debug   bool   `yaml:"debug"`
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host        string `yaml:"host"`
	Port        int    `yaml:"port"`
	User        string `yaml:"user"`
	Password    string `yaml:"password"`
	Database    string `yaml:"database"`
	Charset     string `yaml:"charset"`
	PoolSize    int    `yaml:"pool_size"`
	PoolRecycle int    `yaml:"pool_recycle"`
}

// CacheConfig holds cache configuration
type CacheConfig struct {
	Enabled     bool    `yaml:"enabled"`
	TTLHours    int     `yaml:"ttl_hours"`
	MaxSizeGB   float64 `yaml:"max_size_gb"`
	GzipEnabled bool    `yaml:"gzip_enabled"`
	Dir         string  `yaml:"dir"`
}

// SpiderDetectorConfig holds spider detector configuration
type SpiderDetectorConfig struct {
	Enabled                bool     `yaml:"enabled"`
	Return404ForNonSpider  bool     `yaml:"return_404_for_non_spider"`
	DNSVerifyEnabled       bool     `yaml:"dns_verify_enabled"`
	DNSVerifyTypes         []string `yaml:"dns_verify_types"`
	DNSTimeout             float64  `yaml:"dns_timeout"`
}

// RawConfig represents the raw YAML structure with environments
type RawConfig struct {
	Default     map[string]interface{} `yaml:"default"`
	Development map[string]interface{} `yaml:"development"`
	Production  map[string]interface{} `yaml:"production"`
}

var globalConfig *Config

// Load loads configuration from the Python config.yaml file
func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var raw RawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	// Determine environment from GIN_MODE or ENV_FOR_DYNACONF
	env := os.Getenv("GIN_MODE")
	if env == "" {
		env = os.Getenv("ENV_FOR_DYNACONF")
	}

	// Select environment config
	var envConfig map[string]interface{}
	if env == "release" || env == "production" {
		envConfig = raw.Production
	} else {
		envConfig = raw.Development
	}

	// Merge default with environment config
	merged := mergeConfig(raw.Default, envConfig)

	// Parse into Config struct
	cfg := &Config{
		Server: ServerConfig{
			Host:    getString(merged, "server.host", "127.0.0.1"),
			Port:    getIntEnv("SERVER_PORT", getInt(merged, "server.port", 8080)),
			Workers: getInt(merged, "server.workers", 1),
			Debug:   getBool(merged, "server.debug", false),
		},
		Database: DatabaseConfig{
			Host:        getEnv("DB_HOST", getString(merged, "database.host", "localhost")),
			Port:        getIntEnv("DB_PORT", getInt(merged, "database.port", 3306)),
			User:        getEnv("DB_USER", getString(merged, "database.user", "root")),
			Password:    getEnv("DB_PASSWORD", getString(merged, "database.password", "")),
			Database:    getEnv("DB_NAME", getString(merged, "database.database", "seo_generator")),
			Charset:     getString(merged, "database.charset", "utf8mb4"),
			PoolSize:    getInt(merged, "database.pool_size", 10),
			PoolRecycle: getInt(merged, "database.pool_recycle", 3600),
		},
		Cache: CacheConfig{
			Enabled:     getBool(merged, "cache.enabled", true),
			TTLHours:    getInt(merged, "cache.ttl_hours", 24),
			MaxSizeGB:   getFloat(merged, "cache.max_size_gb", 10.0),
			GzipEnabled: getBool(merged, "cache.gzip_enabled", true),
			Dir:         "./html_cache",
		},
		SpiderDetector: SpiderDetectorConfig{
			Enabled:               getBool(merged, "spider_detector.enabled", true),
			Return404ForNonSpider: getBool(merged, "spider_detector.return_404_for_non_spider", true),
			DNSVerifyEnabled:      getBool(merged, "spider_detector.dns_verify_enabled", false),
			DNSVerifyTypes:        []string{"baidu", "google", "bing"},
			DNSTimeout:            getFloat(merged, "spider_detector.dns_timeout", 2.0),
		},
	}

	globalConfig = cfg
	return cfg, nil
}

// getEnv returns environment variable value or default
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// getIntEnv returns environment variable as int or default
func getIntEnv(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

// Get returns the global configuration
func Get() *Config {
	return globalConfig
}

// GetCacheDir returns the cache directory path relative to the project root
func GetCacheDir(projectRoot string) string {
	if globalConfig != nil && globalConfig.Cache.Dir != "" {
		if filepath.IsAbs(globalConfig.Cache.Dir) {
			return globalConfig.Cache.Dir
		}
		return filepath.Join(projectRoot, globalConfig.Cache.Dir)
	}
	return filepath.Join(projectRoot, "html_cache")
}

// Helper functions for nested map access
func mergeConfig(base, overlay map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range base {
		result[k] = v
	}
	for k, v := range overlay {
		if baseMap, ok := result[k].(map[string]interface{}); ok {
			if overlayMap, ok := v.(map[string]interface{}); ok {
				result[k] = mergeConfig(baseMap, overlayMap)
				continue
			}
		}
		result[k] = v
	}
	return result
}

func getNestedValue(m map[string]interface{}, path string) interface{} {
	keys := splitPath(path)
	current := m
	for i, key := range keys {
		if i == len(keys)-1 {
			return current[key]
		}
		if next, ok := current[key].(map[string]interface{}); ok {
			current = next
		} else {
			return nil
		}
	}
	return nil
}

func splitPath(path string) []string {
	var result []string
	current := ""
	for _, c := range path {
		if c == '.' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func getString(m map[string]interface{}, path, defaultVal string) string {
	if v := getNestedValue(m, path); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultVal
}

func getInt(m map[string]interface{}, path string, defaultVal int) int {
	if v := getNestedValue(m, path); v != nil {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		}
	}
	return defaultVal
}

func getFloat(m map[string]interface{}, path string, defaultVal float64) float64 {
	if v := getNestedValue(m, path); v != nil {
		switch val := v.(type) {
		case float64:
			return val
		case int:
			return float64(val)
		}
	}
	return defaultVal
}

func getBool(m map[string]interface{}, path string, defaultVal bool) bool {
	if v := getNestedValue(m, path); v != nil {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return defaultVal
}
