# 2026_2_PinPals
Backend проекта Pinterest — команды PinPals

## Состав команды

- [Шерстеникин Никита](https://github.com/Sherstanoi)
- [Каргин Владимир](https://github.com/mrvova1)
- [Шевчук Диана](https://github.com/DinaShevchuk)
- [Бессолицын Александр](https://github.com/Sanken282)

### Ментор
- Новиков Антон [GitHub](https://github.com/Anton211) [Telegram](https://t.me/AntonNovikovA)

## Ссылки и ресурсы
- [Frontend репозиторий](https://github.com/frontend-park-mail-ru/2026_2_PinPals)

# Правила разработки

## Структура веток

### Основные ветки
**Прямые коммиты и слияния без PR запрещены.**
- `main` — Версия проекта, подготовленная к показу на РК.
- `develop` — Основная ветка разработки. Сюда сливаются все новые функции и исправления перед релизом в `main`.

### Временные ветки
Создаются для конкретных задач и впоследствии сливаются в `develop`.
- `feature/...` — Разработка новой функциональности.
- `refactoring/...` — Улучшение и переписывание существующего кода без изменения логики.
- `fix/...` — Исправление багов и ошибок.


## Локальный запуск

1. Скопируйте `.env.example` в `.env` и задайте `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` и `AUTH_JWT_SECRET` (не короче 32 символов).
2. Запустите сервисы командой `docker compose up --build` или `make run`.
3. PostgreSQL доступен сервисам внутри compose-сети на `postgres:5432`. Порты HTTP-сервисов на хосте: `8001` (auth), `8002` (pins), `8003` (users), `8004` (chat).

### Изображения

`templates/pin1.jpg`–`templates/pin30.jpg` — демонстрационные изображения, на которые ссылается seed-миграция `db/migrations/005_insert.sql`. Одноразовый `image_storage_init` копирует их в общий volume `pin_images`; `pin_service` раздаёт их и загруженные изображения через `/images/{filename}`. Аватары профиля и досок загружаются как файлы `multipart/form-data`, а не как переданные клиентом URL.

### OpenAPI

OpenAPI-файлы генерируются workflow `.github/workflows/openapi.yml` и доступны как артефакт GitHub Actions; публикация GitHub Pages выполняется при push в `develop`. Swagger UI не входит в runtime-маршруты сервисов.

### Чаты и WebSocket

`chat_service` доступен на `http://localhost:8004`. Все методы требуют авторизацию через `access_token` (HttpOnly cookie или Bearer-токен). Для HTTP-запросов, изменяющих состояние и использующих cookie, также требуется `X-CSRF-Token`.

- `POST /api/v1/chats` — создать личный чат или вернуть существующий (`{ "user_id": 42 }`).
- `GET /api/v1/chats` — список чатов текущего пользователя с последним сообщением.
- `GET /api/v1/chats/{chatID}/messages?limit=50&before_id=123` — история сообщений; `limit` от 1 до 100, сообщения возвращаются в хронологическом порядке.
- `POST /api/v1/chats/{chatID}/messages` — отправить сообщение HTTP-запросом (`{ "body": "Привет", "related_to": 123 }`). Новое сообщение также рассылается активным WebSocket-клиентам чата.
- `GET /api/v1/chats/{chatID}/ws` — WebSocket для отправки и получения сообщений. Клиент отправляет JSON `{ "type": "message", "body": "Привет", "related_to": 123 }`; сервер рассылает событие `{ "type": "message", "message": { ... } }`. Ошибки соединения возвращаются как `{ "type": "error", "error": "..." }`.

В WebSocket используются ping/pong, ограничение размера входящего сообщения и отдельная очередь записи на соединение. Рассылка выполняется в пределах одного экземпляра `chat_service`; для нескольких экземпляров потребуется внешний broker/pub-sub.

### Request ID и SQL-логи

Каждый HTTP-запрос получает UUID в `X-Request-ID` (валидный UUID из входящего заголовка сохраняется). ID добавляется в request-scoped `slog`-логгер и возвращается в ответе. `pgx.QueryTracer` пишет SQL-текст, длительность, command tag и ошибки с тем же `request_id`; значения SQL-параметров намеренно не логируются.
