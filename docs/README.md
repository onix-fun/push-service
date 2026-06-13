# Push Service Documentation

Push Service — это микросервис для управления токенами устройств и доставки push-уведомлений через различных провайдеров.

## Содержание
- [Архитектура](architecture.md) — Обзор компонентов.
- [Конфигурация](configuration.md) — Настройки провайдеров (FCM, APNS, Web Push).
- [Разработка](development.md) — Сборка и тестирование.
- [API Reference](swagger.yaml) — OpenAPI спецификация.

## Основные возможности
- **Multi-provider:** Поддержка Google FCM, Apple APNS и стандартного Web Push.
- **Device Management:** Регистрация, обновление и деактивация токенов.
- **Async Delivery:** Надежная отправка через очередь сообщений.
- **Event Tracking:** Публикация событий о доставке или ошибках обратно в шину.
