# Разработка и запуск

## Требования
- Go 1.26+
- PostgreSQL
- RabbitMQ

## Команды Makefile
- `make build`: Сборка бинарного файла.
- `make run`: Запуск сервиса.
- `make test`: Запуск unit-тестов.
- `make swagger`: Генерация OpenAPI документации.

## Запуск в Docker
Сервис поставляется с Dockerfile, готовым для использования в K8s или Docker Compose.
```bash
docker build -t push-service .
```
