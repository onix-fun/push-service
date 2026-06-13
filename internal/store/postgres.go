package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/onix-fun/push-service/internal/model"
)

type Store struct{ Pool *pgxpool.Pool }

func Open(ctx context.Context, url string) (*Store, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, err
	}
	s := &Store{Pool: pool}
	_, err = pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS push_devices (
 id TEXT PRIMARY KEY, recipient_id TEXT NOT NULL, provider TEXT NOT NULL, token TEXT NOT NULL,
 active BOOLEAN NOT NULL DEFAULT TRUE, updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE(provider, token)
);
CREATE INDEX IF NOT EXISTS push_devices_recipient_active ON push_devices(recipient_id, active);
CREATE TABLE IF NOT EXISTS push_messages (
 message_id TEXT PRIMARY KEY, status TEXT NOT NULL, attempts BIGINT NOT NULL DEFAULT 0, updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE IF NOT EXISTS push_deliveries (
 message_id TEXT NOT NULL, device_id TEXT NOT NULL, status TEXT NOT NULL, error TEXT,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), PRIMARY KEY(message_id, device_id)
);`)
	if err != nil {
		pool.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) UpsertDevice(ctx context.Context, device model.Device) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO push_devices(id,recipient_id,provider,token,active) VALUES($1,$2,$3,$4,TRUE)
ON CONFLICT(id) DO UPDATE SET recipient_id=EXCLUDED.recipient_id,provider=EXCLUDED.provider,token=EXCLUDED.token,active=TRUE,updated_at=NOW()`,
		device.ID, device.RecipientID, device.Provider, device.Token)
	return err
}

func (s *Store) DeactivateDevice(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `UPDATE push_devices SET active=FALSE,updated_at=NOW() WHERE id=$1`, id)
	return err
}

func (s *Store) Devices(ctx context.Context, recipient string) ([]model.Device, error) {
	rows, err := s.Pool.Query(ctx, `SELECT id,recipient_id,provider,token,active FROM push_devices WHERE recipient_id=$1 AND active=TRUE`, recipient)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []model.Device
	for rows.Next() {
		var d model.Device
		if err := rows.Scan(&d.ID, &d.RecipientID, &d.Provider, &d.Token, &d.Active); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

func (s *Store) StartMessage(ctx context.Context, id string) (bool, error) {
	tag, err := s.Pool.Exec(ctx, `INSERT INTO push_messages(message_id,status) VALUES($1,'processing')
ON CONFLICT(message_id) DO UPDATE SET status='processing',updated_at=NOW()
WHERE push_messages.status='retrying'
   OR (push_messages.status='processing' AND push_messages.updated_at < NOW() - INTERVAL '5 minutes')`, id)
	return tag.RowsAffected() == 1, err
}

func (s *Store) FinishMessage(ctx context.Context, id, status string, attempts int64) error {
	_, err := s.Pool.Exec(ctx, `UPDATE push_messages SET status=$2,attempts=$3,updated_at=NOW() WHERE message_id=$1`, id, status, attempts)
	return err
}

func (s *Store) SaveDelivery(ctx context.Context, e model.DeliveryEvent) error {
	_, err := s.Pool.Exec(ctx, `INSERT INTO push_deliveries(message_id,device_id,status,error) VALUES($1,$2,$3,$4)
ON CONFLICT(message_id,device_id) DO UPDATE SET status=EXCLUDED.status,error=EXCLUDED.error,updated_at=NOW()`, e.MessageID, e.DeviceID, e.Status, e.Error)
	return err
}

func (s *Store) DeliveryStatus(ctx context.Context, messageID, deviceID string) (string, error) {
	var status string
	err := s.Pool.QueryRow(ctx, `SELECT status FROM push_deliveries WHERE message_id=$1 AND device_id=$2`, messageID, deviceID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return status, err
}
