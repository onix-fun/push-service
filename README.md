# Push Service

Самостоятельный push gateway с PostgreSQL token registry и RabbitMQ delivery worker.

## Возможности

- HTTP API `/v1/devices` для регистрации FCM, APNs и Web Push устройств;
- команды доставки через RabbitMQ `push.commands.v1`;
- idempotency, retry, delivery history, DLQ и деактивация недействительных токенов;
- Web Push с VAPID;
- FCM HTTP v1 с OAuth2 service account и APNs token authentication;
- роли `api`, `worker`, `all`.

```bash
push-service config validate --config=config/config.example.yaml
push-service serve --config=config/config.example.yaml --role=all
```

Canonical command:

```json
{"message_id":"n-1","recipient_id":"u-1","title":"Title","body":"Body","data":{},"ttl":3600,"collapse_key":"updates"}
```

Лицензия: MIT.
