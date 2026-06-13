package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/onix-fun/push-service/internal/config"
	"github.com/onix-fun/push-service/internal/model"
	"github.com/onix-fun/push-service/internal/provider"
	"github.com/onix-fun/push-service/internal/store"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Worker struct {
	cfg     config.RabbitMQConfig
	store   *store.Store
	gateway *provider.Gateway
	log     *slog.Logger
}

func New(cfg config.RabbitMQConfig, s *store.Store, g *provider.Gateway, l *slog.Logger) *Worker {
	return &Worker{cfg: cfg, store: s, gateway: g, log: l}
}

func (w *Worker) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		if err := w.consume(ctx); err != nil {
			w.log.Error("consumer stopped", "error", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(config.DefaultRetryDelay()):
			}
		}
	}
	return nil
}
func (w *Worker) consume(ctx context.Context) error {
	conn, err := amqp.Dial(w.cfg.URL)
	if err != nil {
		return err
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()
	if err = ch.ExchangeDeclare(w.cfg.CommandExchange, "direct", true, false, false, false, nil); err != nil {
		return err
	}
	if err = ch.ExchangeDeclare(w.cfg.DLQExchange, "direct", true, false, false, false, nil); err != nil {
		return err
	}
	if err = ch.ExchangeDeclare(w.cfg.EventsExchange, "fanout", true, false, false, false, nil); err != nil {
		return err
	}
	if _, err = ch.QueueDeclare(w.cfg.DLQQueue, true, false, false, false, nil); err != nil {
		return err
	}
	if err = ch.QueueBind(w.cfg.DLQQueue, w.cfg.DLQQueue, w.cfg.DLQExchange, false, nil); err != nil {
		return err
	}
	args := amqp.Table{"x-dead-letter-exchange": w.cfg.DLQExchange, "x-dead-letter-routing-key": w.cfg.DLQQueue}
	if _, err = ch.QueueDeclare(w.cfg.CommandQueue, true, false, false, false, args); err != nil {
		return err
	}
	if err = ch.QueueBind(w.cfg.CommandQueue, w.cfg.RoutingKey, w.cfg.CommandExchange, false, nil); err != nil {
		return err
	}
	if err = ch.Qos(w.cfg.Prefetch, 0, false); err != nil {
		return err
	}
	deliveries, err := ch.Consume(w.cfg.CommandQueue, "", false, false, false, false, nil)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case d, ok := <-deliveries:
			if !ok {
				return errors.New("delivery channel closed")
			}
			w.handle(ctx, ch, d)
		}
	}
}
func (w *Worker) handle(ctx context.Context, ch *amqp.Channel, d amqp.Delivery) {
	var command model.Command
	if json.Unmarshal(d.Body, &command) != nil || command.MessageID == "" || command.RecipientID == "" || command.Title == "" {
		_ = d.Nack(false, false)
		return
	}
	fresh, err := w.store.StartMessage(ctx, command.MessageID)
	if err != nil {
		_ = d.Nack(false, true)
		return
	}
	if !fresh {
		_ = d.Ack(false)
		return
	}
	devices, err := w.store.Devices(ctx, command.RecipientID)
	if err != nil {
		_ = d.Nack(false, true)
		return
	}
	transient := false
	for _, device := range devices {
		previous, err := w.store.DeliveryStatus(ctx, command.MessageID, device.ID)
		if err != nil {
			transient = true
			continue
		}
		if previous == "delivered" || previous == "permanent_failure" {
			continue
		}
		event := model.DeliveryEvent{MessageID: command.MessageID, DeviceID: device.ID, RecipientID: command.RecipientID, Provider: device.Provider, Status: "delivered"}
		if err := w.gateway.Send(ctx, device, command); err != nil {
			event.Error = err.Error()
			if errors.Is(err, provider.ErrPermanent) {
				event.Status = "permanent_failure"
				_ = w.store.DeactivateDevice(ctx, device.ID)
			} else {
				event.Status = "transient_failure"
				transient = true
			}
		}
		_ = w.store.SaveDelivery(ctx, event)
		body, _ := json.Marshal(event)
		_ = ch.PublishWithContext(ctx, w.cfg.EventsExchange, "", false, false, amqp.Publishing{ContentType: "application/json", DeliveryMode: amqp.Persistent, Body: body})
	}
	attempts := retryCount(d) + 1
	if transient && attempts < w.cfg.MaxRetries {
		headers := amqp.Table{"x-retry-count": attempts}
		_ = ch.PublishWithContext(ctx, w.cfg.CommandExchange, w.cfg.RoutingKey, false, false, amqp.Publishing{ContentType: "application/json", DeliveryMode: amqp.Persistent, Headers: headers, Body: d.Body})
		_ = w.store.FinishMessage(ctx, command.MessageID, "retrying", attempts)
		_ = d.Ack(false)
		return
	}
	status := "delivered"
	if transient {
		status = "failed"
	}
	_ = w.store.FinishMessage(ctx, command.MessageID, status, attempts)
	if transient {
		_ = d.Nack(false, false)
		return
	}
	_ = d.Ack(false)
}
func retryCount(d amqp.Delivery) int64 {
	switch v := d.Headers["x-retry-count"].(type) {
	case int64:
		return v
	case int32:
		return int64(v)
	}
	return 0
}

var _ = fmt.Sprintf
