package config

import "os"

type Config struct {
	RabbitMQURL string
	Queue       string
	StateFile   string
}

func Load() Config {
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		url = "amqp://guest:guest@localhost:5672/"
	}
	queue := os.Getenv("PUSH_QUEUE")
	if queue == "" {
		queue = "push.commands"
	}
	stateFile := os.Getenv("PUSH_STATE_FILE")
	if stateFile == "" {
		stateFile = "/data/delivered.log"
	}
	return Config{RabbitMQURL: url, Queue: queue, StateFile: stateFile}
}
