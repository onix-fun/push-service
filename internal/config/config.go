package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Service   ServiceConfig  `yaml:"service"`
	Database  DatabaseConfig `yaml:"database"`
	RabbitMQ  RabbitMQConfig `yaml:"rabbitmq"`
	Providers ProviderConfig `yaml:"providers"`
	APIKeys   []APIKey       `yaml:"api_keys"`
}

type ServiceConfig struct {
	HTTPAddr string `yaml:"http_addr"`
}

type DatabaseConfig struct {
	URL string `yaml:"url"`
}

type RabbitMQConfig struct {
	URL             string `yaml:"url"`
	CommandQueue    string `yaml:"command_queue"`
	CommandExchange string `yaml:"command_exchange"`
	RoutingKey      string `yaml:"routing_key"`
	DLQExchange     string `yaml:"dlq_exchange"`
	DLQQueue        string `yaml:"dlq_queue"`
	EventsExchange  string `yaml:"events_exchange"`
	Prefetch        int    `yaml:"prefetch"`
	MaxRetries      int64  `yaml:"max_retries"`
}

type ProviderConfig struct {
	FCM     FCMProvider     `yaml:"fcm"`
	APNS    APNSProvider    `yaml:"apns"`
	WebPush WebPushProvider `yaml:"web_push"`
}

type FCMProvider struct {
	Enabled     bool   `yaml:"enabled"`
	Endpoint    string `yaml:"endpoint"`
	ProjectID   string `yaml:"project_id"`
	Credentials string `yaml:"credentials"`
}

type APNSProvider struct {
	Enabled    bool   `yaml:"enabled"`
	Endpoint   string `yaml:"endpoint"`
	TeamID     string `yaml:"team_id"`
	KeyID      string `yaml:"key_id"`
	BundleID   string `yaml:"bundle_id"`
	PrivateKey string `yaml:"private_key"`
}

type WebPushProvider struct {
	Enabled    bool   `yaml:"enabled"`
	PublicKey  string `yaml:"public_key"`
	PrivateKey string `yaml:"private_key"`
	Subject    string `yaml:"subject"`
}

type APIKey struct {
	Name   string   `yaml:"name"`
	Value  string   `yaml:"value"`
	Scopes []string `yaml:"scopes"`
}

var envPattern = regexp.MustCompile(`\$\{([A-Z0-9_]+)\}`)

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	expanded := envPattern.ReplaceAllStringFunc(string(data), func(raw string) string {
		return os.Getenv(envPattern.FindStringSubmatch(raw)[1])
	})
	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return Config{}, fmt.Errorf("decode config: %w", err)
	}
	resolve := func(value string) (string, error) {
		if !strings.HasPrefix(value, "file:") {
			return value, nil
		}
		data, err := os.ReadFile(strings.TrimPrefix(value, "file:"))
		return strings.TrimSpace(string(data)), err
	}
	cfg.Database.URL, err = resolve(cfg.Database.URL)
	if err != nil {
		return Config{}, err
	}
	cfg.RabbitMQ.URL, err = resolve(cfg.RabbitMQ.URL)
	if err != nil {
		return Config{}, err
	}
	for index := range cfg.APIKeys {
		cfg.APIKeys[index].Value, err = resolve(cfg.APIKeys[index].Value)
		if err != nil {
			return Config{}, err
		}
	}
	if cfg.Providers.FCM.Enabled {
		cfg.Providers.FCM.Credentials, err = resolve(cfg.Providers.FCM.Credentials)
		if err != nil {
			return Config{}, err
		}
	}
	if cfg.Providers.APNS.Enabled {
		cfg.Providers.APNS.PrivateKey, err = resolve(cfg.Providers.APNS.PrivateKey)
		if err != nil {
			return Config{}, err
		}
	}
	cfg.Providers.WebPush.PrivateKey, err = resolve(cfg.Providers.WebPush.PrivateKey)
	if err != nil {
		return Config{}, err
	}
	return cfg, cfg.Validate()
}

func (c Config) Validate() error {
	if c.Service.HTTPAddr == "" || c.Database.URL == "" || c.RabbitMQ.URL == "" {
		return errors.New("service.http_addr, database.url and rabbitmq.url are required")
	}
	if c.RabbitMQ.CommandQueue == "" || c.RabbitMQ.CommandExchange == "" || c.RabbitMQ.RoutingKey == "" || c.RabbitMQ.DLQExchange == "" || c.RabbitMQ.DLQQueue == "" || c.RabbitMQ.EventsExchange == "" {
		return errors.New("rabbitmq topology is incomplete")
	}
	if c.RabbitMQ.Prefetch <= 0 || c.RabbitMQ.MaxRetries <= 0 {
		return errors.New("rabbitmq.prefetch and max_retries must be positive")
	}
	if len(c.APIKeys) == 0 {
		return errors.New("at least one api key is required")
	}
	if c.Providers.FCM.Enabled && (c.Providers.FCM.ProjectID == "" || c.Providers.FCM.Credentials == "") {
		return errors.New("providers.fcm project_id and credentials are required when enabled")
	}
	if c.Providers.APNS.Enabled && (c.Providers.APNS.TeamID == "" || c.Providers.APNS.KeyID == "" || c.Providers.APNS.BundleID == "" || c.Providers.APNS.PrivateKey == "") {
		return errors.New("providers.apns team_id, key_id, bundle_id and private_key are required when enabled")
	}
	if c.Providers.WebPush.Enabled && (c.Providers.WebPush.PublicKey == "" || c.Providers.WebPush.PrivateKey == "" || c.Providers.WebPush.Subject == "") {
		return errors.New("providers.web_push public_key, private_key and subject are required when enabled")
	}
	return nil
}

func DefaultRetryDelay() time.Duration { return 5 * time.Second }
