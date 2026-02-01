package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds the configuration for the crawler service
type Config struct {
	Frontier    FrontierConfig
	Storage     StorageConfig
	Politeness  PolitenessConfig
	LLM         LLMConfig
}

type LLMConfig struct {
	Provider string
	BaseURL  string
	Model    string
	APIKey   string
}

// FrontierConfig holds URL frontier specific configuration
type FrontierConfig struct {
	MaxURLsInMemory    int
	MaxRetries         int
	DefaultPriority    int
	CleanupInterval    time.Duration
	PolitenessDelay    time.Duration
	MaxConcurrency     int
	EnablePersistence  bool
}

// StorageConfig holds storage configuration
type StorageConfig struct {
	RedisURL      string
	RedisPassword string
	RedisDB       int
}

// PolitenessConfig holds politeness manager configuration
type PolitenessConfig struct {
	DefaultMinDelay          time.Duration
	DefaultDomainConcurrency int
	GlobalMaxConcurrency     int
	RequestQueueSize         int
	DefaultRequestTimeout    time.Duration
	CleanupInterval          time.Duration
	DomainStateExpiry        time.Duration
	RobotsCacheDuration      time.Duration
	EnableRobotsCheck        bool
	UserAgent                string
}

// Load loads configuration from environment variables with defaults
func Load() *Config {
	return &Config{
		Frontier: FrontierConfig{
			MaxURLsInMemory:    GetIntEnv("FRONTIER_MAX_URLS", 10000),
			MaxRetries:         GetIntEnv("FRONTIER_MAX_RETRIES", 3),
			DefaultPriority:    GetIntEnv("FRONTIER_DEFAULT_PRIORITY", 1),
			CleanupInterval:    GetDurationEnv("FRONTIER_CLEANUP_INTERVAL", 5*time.Minute),
			PolitenessDelay:    GetDurationEnv("FRONTIER_POLITENESS_DELAY", 1*time.Second),
			MaxConcurrency:     GetIntEnv("FRONTIER_MAX_CONCURRENCY", 10),
			EnablePersistence:  GetBoolEnv("FRONTIER_ENABLE_PERSISTENCE", true),
		},
		Storage: StorageConfig{
			RedisURL:      GetStringEnv("REDIS_URL", "localhost:6379"),
			RedisPassword: GetStringEnv("REDIS_PASSWORD", ""),
			RedisDB:       GetIntEnv("REDIS_DB", 0),
		},
		Politeness: PolitenessConfig{
			DefaultMinDelay:          GetDurationEnv("POLITENESS_DEFAULT_MIN_DELAY", 1*time.Second),
			DefaultDomainConcurrency: GetIntEnv("POLITENESS_DEFAULT_DOMAIN_CONCURRENCY", 2),
			GlobalMaxConcurrency:     GetIntEnv("POLITENESS_GLOBAL_MAX_CONCURRENCY", 100),
			RequestQueueSize:         GetIntEnv("POLITENESS_REQUEST_QUEUE_SIZE", 1000),
			DefaultRequestTimeout:    GetDurationEnv("POLITENESS_DEFAULT_REQUEST_TIMEOUT", 30*time.Second),
			CleanupInterval:          GetDurationEnv("POLITENESS_CLEANUP_INTERVAL", 10*time.Minute),
			DomainStateExpiry:        GetDurationEnv("POLITENESS_DOMAIN_STATE_EXPIRY", 1*time.Hour),
			RobotsCacheDuration:      GetDurationEnv("POLITENESS_ROBOTS_CACHE_DURATION", 24*time.Hour),
			EnableRobotsCheck:        GetBoolEnv("POLITENESS_ENABLE_ROBOTS_CHECK", true),
			UserAgent:                GetStringEnv("POLITENESS_USER_AGENT", "SearchEngine-Crawler/1.0"),
		},
		LLM: LLMConfig{
			Provider: GetStringEnv("LLM_PROVIDER", "ollama"),
			BaseURL:  GetStringEnv("LLM_BASE_URL", ""),
			Model:    GetStringEnv("LLM_MODEL", "qwen3:1.7b"),
			APIKey:   GetStringEnv("LLM_API_KEY", ""),
		},
	}
}

func GetStringEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func GetIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func GetBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func GetDurationEnv(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}