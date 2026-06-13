package main

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"push-service/internal/config"
)

type PushCommand struct {
	EventID       string                 `json:"eventId"`
	SourceEventID string                 `json:"sourceEventId"`
	RecipientID   string                 `json:"recipientId"`
	Type          string                 `json:"type"`
	Title         string                 `json:"title"`
	Body          string                 `json:"body"`
	EntityType    string                 `json:"entityType,omitempty"`
	EntityID      string                 `json:"entityId,omitempty"`
	Data          map[string]interface{} `json:"data"`
}

type IdempotencyStore struct {
	mu        sync.Mutex
	delivered map[string]struct{}
	file      *os.File
}

func openIdempotencyStore(path string) (*IdempotencyStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	store := &IdempotencyStore{delivered: map[string]struct{}{}, file: file}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		store.delivered[scanner.Text()] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		_ = file.Close()
		return nil, err
	}
	return store, nil
}

func (s *IdempotencyStore) Contains(eventID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.delivered[eventID]
	return exists
}

func (s *IdempotencyStore) MarkDelivered(eventID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.delivered[eventID]; exists {
		return nil
	}
	if _, err := s.file.WriteString(eventID + "\n"); err != nil {
		return err
	}
	if err := s.file.Sync(); err != nil {
		return err
	}
	s.delivered[eventID] = struct{}{}
	return nil
}

func connectRabbitMQ(url string) (*amqp091.Connection, error) {
	var conn *amqp091.Connection
	var err error
	for i := 0; i < 10; i++ {
		conn, err = amqp091.Dial(url)
		if err == nil {
			return conn, nil
		}
		log.Printf("failed to connect to RabbitMQ, retrying in 3 seconds: %v", err)
		time.Sleep(3 * time.Second)
	}
	return nil, err
}

func main() {
	cfg := config.Load()
	store, err := openIdempotencyStore(cfg.StateFile)
	if err != nil {
		log.Fatalf("failed to open idempotency store: %v", err)
	}
	defer store.file.Close()

	conn, err := connectRabbitMQ(cfg.RabbitMQURL)
	if err != nil {
		log.Fatalf("failed to connect to RabbitMQ after retries: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("failed to open channel: %v", err)
	}
	defer ch.Close()

	if err := ch.ExchangeDeclare("push.dlx", "direct", true, false, false, false, nil); err != nil {
		log.Fatalf("failed to declare dead letter exchange: %v", err)
	}
	if _, err := ch.QueueDeclare("push.dlq", true, false, false, false, nil); err != nil {
		log.Fatalf("failed to declare dead letter queue: %v", err)
	}
	if err := ch.QueueBind("push.dlq", "push.dlq", "push.dlx", false, nil); err != nil {
		log.Fatalf("failed to bind dead letter queue: %v", err)
	}
	queue, err := ch.QueueDeclare(cfg.Queue, true, false, false, false, amqp091.Table{
		"x-dead-letter-exchange":    "push.dlx",
		"x-dead-letter-routing-key": "push.dlq",
	})
	if err != nil {
		log.Fatalf("failed to declare queue: %v", err)
	}
	if err := ch.Qos(10, 0, false); err != nil {
		log.Fatalf("failed to configure qos: %v", err)
	}
	messages, err := ch.Consume(queue.Name, "", false, false, false, false, nil)
	if err != nil {
		log.Fatalf("failed to register consumer: %v", err)
	}

	log.Printf("push-service stub started; waiting for commands")
	for delivery := range messages {
		var command PushCommand
		if err := json.Unmarshal(delivery.Body, &command); err != nil {
			log.Printf("invalid push command JSON: %v", err)
			_ = delivery.Nack(false, false)
			continue
		}
		if command.EventID == "" || command.RecipientID == "" || command.Type == "" || command.Title == "" || command.Body == "" {
			log.Printf("invalid push command: required fields are missing")
			_ = delivery.Nack(false, false)
			continue
		}
		if store.Contains(command.EventID) {
			_ = delivery.Ack(false)
			continue
		}

		log.Printf(
			"push delivered by stub eventId=%s recipientId=%s type=%s entityType=%s entityId=%s",
			command.EventID, command.RecipientID, command.Type, command.EntityType, command.EntityID,
		)
		if err := store.MarkDelivered(command.EventID); err != nil {
			log.Printf("failed to persist delivered event: %v", err)
			_ = delivery.Nack(false, true)
			continue
		}
		_ = delivery.Ack(false)
	}
}
