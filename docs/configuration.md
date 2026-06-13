# Конфигурация Push Service

Сервис настраивается через YAML файл (по умолчанию `config/config.yaml`) или переменные окружения.

## Параметры конфигурации

### Service
- `http_addr`: Адрес и порт HTTP сервера (например, `:8080`).

### Database
Настройки подключения к PostgreSQL.
- `url`: DSN подключения.
- `auto_migrate`: Автоматически применять миграции при старте (true/false).
- `migration_path`: Путь к файлам миграций (например, `file://migrations`).

### RabbitMQ
- `url`: Адрес RabbitMQ.
- `command_exchange`: Обменник для входящих команд на отправку.
- `command_queue`: Очередь для команд.
- `routing_key`: Ключ маршрутизации для команд.
- `dlq_exchange`: Обменник для Dead Letter Queue.
- `dlq_queue`: Очередь для Dead Letter Queue.
- `events_exchange`: Обменник для публикации событий о доставке/ошибках.
- `prefetch`: Лимит одновременно обрабатываемых сообщений.
- `max_retries`: Максимальное количество попыток доставки.

### Providers
Настройки провайдеров доставки уведомлений.

#### FCM (Firebase Cloud Messaging)
- `enabled`: Включить провайдер.
- `project_id`: ID проекта Firebase.
- `credentials`: Путь к файлу сервисного аккаунта (поддерживается префикс `file:`).

#### APNS (Apple Push Notification service)
- `enabled`: Включить провайдер.
- `team_id`: Apple Team ID.
- `key_id`: Key ID.
- `bundle_id`: App Bundle ID.
- `private_key`: Путь к файлу закрытого ключа `.p8`.

#### Web Push
- `enabled`: Включить провайдер.
- `public_key`: VAPID публичный ключ.
- `private_key`: VAPID приватный ключ.
- `subject`: URL или mailto адрес для идентификации отправителя.

### API Keys
Статические ключи для авторизации API запросов.
- `name`: Имя потребителя.
- `value`: Значение ключа.
- `scopes`: Разрешения (например, `devices:write`, `devices:read`).
