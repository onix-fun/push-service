package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadExpandsEnvironmentAndValidates(t *testing.T) {
	t.Setenv("DB", "postgres://example")
	t.Setenv("RMQ", "amqp://example")
	t.Setenv("KEY", "secret")
	path := filepath.Join(t.TempDir(), "config.yaml")
	data := []byte(`service: {http_addr: ":8080"}
database: {url: "${DB}"}
rabbitmq:
  url: "${RMQ}"
  command_exchange: commands
  command_queue: commands.q
  routing_key: commands.v1
  dlq_exchange: dlq
  dlq_queue: dlq.q
  events_exchange: events
  prefetch: 1
  max_retries: 2
providers: {}
api_keys: [{name: test, value: "${KEY}", scopes: ["*"]}]
`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.APIKeys[0].Value != "secret" {
		t.Fatalf("secret was not expanded")
	}
}
