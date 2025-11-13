package config

import (
	"time"
)

// Конфигурация сервера

type ServerConfig struct {
	Listen          string        `mapstructure:"listen"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

// Конфигурация логирования

const (
	LogLevelInfo  LogLevel = "info"
	LogLevelDebug LogLevel = "debug"
	LogLevelError LogLevel = "error"
)

type LogLevel string

type LoggingConfig struct {
	Level LogLevel `mapstructure:"log_level"`
}

// Конфигурация апстримов

const (
	BalancingAlgorithmRoundRobin       BalancingAlgorithm = "round_robin"
	BalancingAlgorithmLeastConnections BalancingAlgorithm = "least_connections"
)

type BalancingAlgorithm string

type HealthCheck struct {
	Interval        time.Duration `mapstructure:"interval"`
	DisableDuration time.Duration `mapstructure:"disable_duration"`
}

type UpstreamConfig struct {
	Algorithm   BalancingAlgorithm `mapstructure:"algorithm"`
	Servers     []string           `mapstructure:"servers"`
	HealthCheck *HealthCheck       `mapstructure:"health_check"`
}

type UpstreamsConfig map[string]*UpstreamConfig

// Конфигурация локаций

type CacheConfig struct {
	TTL  time.Duration `mapstructure:"ttl"`
	Size int           `mapstructure:"size"`
	Key  string        `mapstructure:"key"`
}

type RateLimitConfig struct {
	RPM  int    `mapstructure:"rpm"`
	Key  string `mapstructure:"key"`
	Size int    `mapstructure:"size"`
}

type LocationConfig struct {
	Path             string           `mapstructure:"path"`
	ProxyPass        string           `mapstructure:"proxy_pass"`
	Timeout          time.Duration    `mapstructure:"timeout"`
	Root             string           `mapstructure:"root"`
	Cache            *CacheConfig     `mapstructure:"cache"`
	RateLimit        *RateLimitConfig `mapstructure:"rate_limit"`
	WebSocketEnabled bool             `mapstructure:"websocket"`
}

type LocationsConfig []*LocationConfig

type AdminConfig struct {
	Listen string `mapstructure:"listen"`
}

// Основной конфиг

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Logging   LoggingConfig   `mapstructure:"logging"`
	Upstreams UpstreamsConfig `mapstructure:"upstreams"`
	Locations LocationsConfig `mapstructure:"locations"`
	Admin     AdminConfig     `mapstructure:"admin"`
}

// TODO multi errors в валидации конфига
// TODO добавить дефолт логгирование
