# PinPals Backend — Project Context for AI Coding Agents

> Этот файл предназначен для передачи контекста кодовым AI-агентам перед анализом, исправлением или генерацией кода. Он описывает backend-репозиторий PinPals, его архитектуру, технические решения и соглашения разработки. Это не замена исходному коду: перед изменением поведения всегда сверяйтесь с актуальными файлами и тестами.

## 1. Краткое описание проекта

**PinPals** — backend Pinterest-подобного приложения команды `go-park-mail-ru/2026_2_PinPals`. Backend реализован на Go и разделён на четыре HTTP-сервиса:

- `auth_service` — регистрация, вход, JWT, выход и отзыв токенов;
- `user_service` — профиль текущего пользователя;
- `pin_service` — пины, доски, связи пинов с досками, лайки, теги и изображения;
- `chat_service` — личные чаты, история сообщений и WebSocket-доставка.

Сервисы — отдельные процессы/контейнеры, но используют одну PostgreSQL базу данных и общий набор инфраструктурных пакетов из `pkg/`. Каждый экземпляр сервиса при старте создаёт собственный `pgxpool.Pool`.

Основные инфраструктурные зависимости:

- Go (`go.mod` задаёт `go 1.27`);
- HTTP — стандартный `net/http`, `http.ServeMux` с method/path patterns;
- PostgreSQL 16;
- `github.com/jackc/pgx/v5` и `pgxpool`;
- SQL migrations — Tern (`db/tern.conf`, `db/migrations/`);
- JWT — `github.com/golang-jwt/jwt/v5`;
- валидация DTO — `github.com/go-playground/validator/v10` там, где он используется, плюс явные `Validate()` методы;
- WebSocket — `github.com/gorilla/websocket`;
- UUID — `github.com/google/uuid`;
- логирование — `log/slog` с JSON handler;
- тесты — стандартный `testing`, `httptest`, `testify`, `pgxmock`, `gomock`.

## 2. Структура репозитория

```text
.
├── auth_service/
│   ├── app.go                  # composition root и HTTP routes
│   ├── delivery/http/          # handlers, transport DTO, mocks
│   ├── domain/                 # сущности и доменные ошибки
│   ├── repository/             # PostgreSQL реализация
│   └── usecase/                # сценарии авторизации и интерфейсы репозитория
├── user_service/
│   ├── app.go
│   ├── delivery/http/
│   ├── domain/
│   ├── repository/
│   └── usecase/
├── pin_service/
│   ├── app.go
│   ├── delivery/http/          # pins, desks, likes, DTO и mocks
│   ├── domain/
│   ├── repository/
│   └── usecase/                # pin, desk и like services
├── chat_service/
│   ├── app.go
│   ├── delivery/http/          # handlers, DTO, WebSocket Hub и client pumps
│   ├── domain/                 # Chat, Message, MessageView, errors
│   ├── repository/             # PostgreSQL chat/message queries
│   └── usecase/                # бизнес-сценарии чатов и сообщений
├── cmd/                        # main package для каждого сервиса
├── pkg/
│   ├── config/                 # env-конфигурация и DSN
│   ├── database/               # единый PostgreSQL pool factory
│   ├── httpx/                  # JSON decode и HTTP response helpers
│   ├── images/                 # загрузка, хранение и URL изображений
│   ├── middleware/             # JWT, CSRF, CORS, Request ID, logging, security
│   └── server/                 # общий HTTP lifecycle и server constructor
├── db/
│   ├── migrations/             # последовательные Tern up/down migrations
│   └── tern.conf
├── docs/                       # сгенерированные OpenAPI/Swagger материалы
├── templates/                  # seed/demo images
├── site/                       # статическая страница/демо
├── Dockerfile
├── Dockerfile.migrate
├── docker-compose.yaml
├── Makefile
├── go.mod / go.sum
└── README.md
```

### Роль основных слоёв

Обычный путь HTTP-запроса:

```text
cmd/<service>/main.go
    -> <service>.Run
        -> pkg/server.Run
            -> middleware chain
                -> delivery/http Handler
                    -> usecase Service
                        -> repository interface
                            -> PostgreSQL Repository
                                -> pgxpool
```

- `cmd/<service>/main.go` — только запуск соответствующего сервиса; бизнес-логики здесь быть не должно.
- `<service>/app.go` — composition root: создаёт token manager, repository, usecase, handler и регистрирует маршруты.
- `delivery/http` — транспортный слой. Он разбирает path/query/body, проверяет формат HTTP-входа, вызывает use-case, преобразует domain model в response DTO и отображает ошибки в HTTP status.
- `domain` — сущности, правила и доменные ошибки. Не должен зависеть от HTTP, PostgreSQL или WebSocket.
- `usecase` — прикладные сценарии и валидация бизнес-правил. Интерфейсы репозиториев обычно объявляются рядом с use-case, который ими пользуется.
- `repository` — SQL и преобразование строк PostgreSQL в domain/read models. Не должен принимать HTTP request/response объекты.
- `pkg/` — общий технический код, не привязанный к одному сервису.

Сохраняйте направление зависимостей: delivery -> usecase -> domain/interfaces; repository реализует интерфейсы usecase. Не переносите SQL в handler и не помещайте HTTP-статусы в domain.

## 3. Запуск сервисов и инициализация БД

Все сервисы используют общий запуск:

```go
func Run(ctx context.Context) error {
    return server.Run(ctx, RegisterRoutes)
}
```

`pkg/server/lifecycle.go` централизованно:

1. читает конфигурацию через `config.MustLoad()`;
2. создаёт JSON `slog.Logger`;
3. создаёт пул через `database.NewPostgresPool(ctx, cfg.Postgres.DSN(), logger)`;
4. регистрирует `/healthz` и service routes;
5. подключает общие HTTP middleware;
6. запускает `http.Server`;
7. выполняет graceful shutdown и вызывает route cleanup callback до закрытия пула.

**Не добавляйте `SetupPostgres`/`setupDB` wrapper в отдельный сервис.** Общая реализация находится в `pkg/database.NewPostgresPool`; общий lifecycle — в `pkg/server.Run`. Это гарантирует единый QueryTracer и одинаковую проверку подключения для всех сервисов. `RegisterRoutes` возвращает `func()` для ресурсов, которым нужна очистка; сейчас это важно для WebSocket Hub в `chat_service`.

`database.NewPostgresPool`:

- разбирает DSN через `pgxpool.ParseConfig`;
- ставит `middleware.NewQueryLogger()` в `ConnConfig.Tracer`;
- создаёт `pgxpool`;
- проверяет доступность PostgreSQL через `Ping` с timeout 5 секунд;
- закрывает пул, если проверка соединения не прошла.

`server.New` задаёт HTTP timeouts (`ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`). Не удаляйте ограничения времени без явной причины.

## 4. Конфигурация

Конфигурация читается из environment variables в `pkg/config/config.go`; пример — `.env.example`.

| Переменная | Назначение |
|---|---|
| `APP_PORT` | HTTP-порт процесса внутри контейнера (обычно `8000`) |
| `APP_BASE_URL` | Публичный базовый URL сервиса для URL изображений/ресурсов |
| `APP_ENV` | Режим приложения |
| `LOG_LEVEL` | `debug`, `info`, `warn` или `error` для `slog` |
| `IMAGE_DIR` | Директория хранения изображений внутри контейнера |
| `POSTGRES_HOST` | Хост PostgreSQL; внутри Compose — `postgres` |
| `POSTGRES_INTERNAL_PORT` | Порт PostgreSQL внутри сети (обычно `5432`) |
| `POSTGRES_USER` | Пользователь БД; обязателен |
| `POSTGRES_PASSWORD` | Пароль БД; обязателен |
| `POSTGRES_DB` | Имя БД; обязательно |
| `POSTGRES_SSLMODE` | Режим SSL, локально обычно `disable` |
| `AUTH_JWT_SECRET` | Секрет подписи JWT; минимум 32 символа |
| `AUTH_TOKEN_TTL` | Время жизни JWT, например `24h` |
| `CORS_ALLOWED_ORIGINS` | Список разрешённых origin через запятую |

Не записывайте реальные пароли/JWT secrets в код, тестовые фикстуры, документацию или commit. Используйте `.env` локально, он не должен попадать в git.

У сервисов свой `APP_BASE_URL` может отличаться — сверяйте переменные окружения в Compose и `.env.example`, когда меняете построение публичных ссылок.

## 5. HTTP middleware, авторизация и безопасность

Общие middleware расположены в `pkg/middleware`:

- `RequestLogger` — единый request-id middleware;
- `Access` — обязательная JWT-аутентификация;
- `OptionalAccess` — анонимный доступ с проверкой токена, если он передан;
- `CSRF` — double-submit CSRF protection для state-changing запросов с cookie;
- `CORS` — allowlist origins;
- `SecurityHeaders` — security response headers.

### JWT и пользовательский контекст

`TokenManager` выпускает HS256 JWT с `user_id`, `jti`, `sub`, `iat`, `exp`. В браузере токен хранится в HttpOnly cookie `access_token`; для обратной совместимости поддерживается `Authorization: Bearer ...`. CSRF cookie называется `csrf_token`, значение для state-changing HTTP-запросов передаётся в `X-CSRF-Token`.

После успешной проверки JWT middleware кладёт `user_id` в context. В handler используйте `middleware.UserIDFromContext(r.Context())`; не принимайте `user_id` текущего пользователя из body/query — автор должен определяться только по auth context. `OptionalAccess` используется только на маршрутах, где анонимность предусмотрена.

Для защищённых cookie-based POST/PATCH/DELETE маршрутов сохраняйте `middleware.CSRF(...)` вокруг authenticated handler. Для Bearer auth CSRF middleware пропускает проверку, если auth cookie не используется.

### Request ID и контекстный logger

Единый механизм находится в `pkg/middleware`:

- `middleware.RequestLogger(logger)` — единственный HTTP middleware, создающий Request ID;
- заголовок — `X-Request-ID`;
- корректный входящий UUID нормализуется и сохраняется; отсутствующий/некорректный ID заменяется новым UUID;
- ID возвращается клиенту в response header;
- `RequestContext` добавляет ID и request-scoped `slog.Logger` в context;
- `LoggerFromContext(ctx)` возвращает scoped logger;
- `RequestIDFromContext(ctx)` возвращает текущий ID.

Порядок middleware в `pkg/server/lifecycle.go` важен: RequestLogger оборачивает остальные handlers, чтобы ID и logger были доступны всей обработке запроса. Все логи внутри HTTP request path должны использовать `middleware.LoggerFromContext(ctx)` и `InfoContext/WarnContext/ErrorContext`, а не глобальный logger без `request_id`. Логи старта/остановки сервиса могут использовать application logger напрямую.

`pkg/database.NewPostgresPool` устанавливает `pgx.QueryTracer`. `TraceQueryStart` сохраняет SQL и время начала в context; `TraceQueryEnd` берёт logger из того же context и пишет SQL, длительность, command tag и ошибку. **SQL args не логируются**, потому что там могут быть пароли, токены, личные данные и текст сообщений.

### Request ID внутри WebSocket

HTTP handshake получает обычный request ID. Он используется для логов открытия/закрытия WebSocket-соединения. Долгоживущее соединение может принимать множество сообщений, поэтому каждое входящее сообщение должно начинаться с нового operation context через `middleware.OperationContext(parentCtx)`. Новый ID и scoped logger передаются в обработчик сообщения, use-case и pgx, чтобы все логи одной операции (включая SQL) имели один ID.

Не переиспользуйте handshake Request ID для всех сообщений WebSocket. Не создавайте ID отдельно в repository: источник контекста — транспортный слой.

## 6. Валидация и HTTP JSON

В проекте валидация многоуровневая:

1. **Transport/delivery validation** — формат входящих HTTP данных, path parameters, query parameters, обязательные поля, длины, enum/range и корректность JSON. Ошибка обычно возвращается как HTTP 400 до вызова use-case.
2. **Use-case validation** — повторная защита бизнес-операции от некорректных аргументов независимо от транспорта (HTTP, WebSocket, другой адаптер, unit test).
3. **Database constraints** — окончательная целостность данных: FK, unique, check constraints и т. п.

Эти уровни не заменяют друг друга. Не удаляйте use-case checks только потому, что DTO проверяется в handler. И наоборот, наличие проверки в use-case не оправдывает пропуск transport validation.

### JSON

Используйте `httpx.DecodeJSON(w, r, &dto)` из `pkg/httpx/json.go`. Он:

- проверяет `Content-Type`, когда он указан (допускается `application/json`);
- ограничивает тело 2 MiB;
- включает `DisallowUnknownFields`;
- требует ровно одно JSON-значение;
- записывает HTTP error самостоятельно и возвращает ошибку вызывающему handler.

После decode вызывайте DTO `Validate()`/нормализацию. Не декодируйте JSON второй раз и не обходите лимит тела без причины. Для больших бинарных загрузок используйте multipart и специализированный image handler.

### Chat transport validation

В `chat_service/delivery/http` текущая реализация проверяет до use-case:

- `CreateChatRequest.Validate()` — положительный `user_id` собеседника;
- `pathID(r, "chatID")` — path ID должен быть положительным целым числом;
- `ParseListMessagesQuery` — `limit` один раз, целое число в диапазоне 1–`domain.MaxMessagesPerPage` (100; по умолчанию 50); `before_id` один раз, целое неотрицательное число (по умолчанию 0);
- `SendMessageRequest.Validate()` через `normalized()` — тип события, непустой trimmed body, корректный UTF-8, длина не более `domain.MaxMessageLength` (4000 Unicode code points), `related_to` должен быть положительным;
- WebSocket frame — валидный JSON ровно одного значения, только известные поля, `type == "message"`, затем та же проверка payload.

`ListMessages` должен валидировать **и path `chatID`, и query pagination** перед `service.ListMessages`. Не переносите эти проверки только в use-case. Тесты transport должны подтверждать, что при невалидном вводе service вообще не вызывается.

При добавлении нового endpoint проверьте path/query/body отдельно и добавьте handler tests на некорректные значения. Не используйте `strconv.Atoi` без проверки ошибки и допустимого диапазона.

## 7. PostgreSQL и миграции

Все запросы идут через `pgxpool`; используйте `context.Context` из запроса/операции во всех `Query`, `QueryRow`, `Exec`. SQL должен быть параметризован `$1`, `$2`, ...; никогда не конкатенируйте пользовательский ввод в SQL.

Основные migration files (порядок Tern):

- `001_functions.sql` — DB functions;
- `002_user.sql`, `003_password.sql` — пользователи/пароли;
- `004_pin.sql`, `005_insert.sql` — pins и seed data;
- `006_desk.sql` — доски;
- `007_pin-like.sql` — лайки;
- `008_pin-desk-relation.sql` — связи пинов и досок;
- `009_tag.sql`, `010_pin-tag-relation.sql` — теги;
- `011_revoked_jwt_token.sql` — отозванные JWT;
- `012_subscription.sql` — подписки;
- `013_chat.sql` — личные чаты и ограничения участников;
- `014_chat-message.sql` — сообщения и ответы;
- `015_FTS_Search.sql` — full-text search;
- `016_chat_indexes.sql` — индексы чатов и сообщений.

Новые изменения схемы добавляйте новой последовательной migration с up/down секциями. Не редактируйте старую migration, если она уже применялась на совместной/production базе: создайте новую migration. Проверяйте порядок FK, индексы под фактические query patterns и rollback.

### Схема чата

`chat` хранит пару участников, имя, timestamps и soft-delete поля. Уникальность пары пользователей и constraint `user_a_id < user_b_id` задают канонический порядок: `(A, B)` и `(B, A)` не создают разные чаты; чат с самим собой запрещён.

`chat_message` хранит `message_id`, `chat_id`, `author_id`, `body`, `related_to`, timestamps и soft-delete поля. Автор профиля не денормализован в каждой записи сообщения.

В Go:

- `domain.Message` — сохраняемые поля сообщения: ID, ChatID, AuthorID, Body, RelatedTo, CreatedAt;
- `domain.MessageView` — read model: Message + имя и tag автора, заполняемые SQL JOIN;
- `delivery/http.MessageResponse` — внешний JSON DTO с `author_name` и `author_tag`.

При новых API расширениях сохраняйте разделение persistent entity, read model и transport DTO. Не добавляйте `AuthorName`/`AuthorTag` в таблицу `chat_message`, если нет отдельного осознанного требования хранить исторический snapshot автора.

Repository `chat_service/repository/postgres.go` делает JOIN к `"user"` при чтении сообщений и последнего сообщения в списке чатов. Cursor pagination (`before_id`) выполняется по `message_id`; DB выбирает newest-first для эффективного `LIMIT`, API разворачивает выбранную страницу в chronological order.

## 8. Чаты и WebSocket

Маршруты `chat_service` (base path `/api/v1`):

| Метод | Маршрут | Назначение |
|---|---|---|
| POST | `/chats` | Создать личный чат/получить существующий; body `{ "user_id": 42 }` |
| GET | `/chats` | Список чатов текущего пользователя и последнее сообщение |
| GET | `/chats/{chatID}/messages?limit=50&before_id=123` | История сообщений |
| POST | `/chats/{chatID}/messages` | Создать сообщение через HTTP |
| GET | `/chats/{chatID}/ws` | WebSocket соединение чата |

HTTP маршруты требуют auth; POST маршруты защищены CSRF при использовании cookie.

WebSocket client message:

```json
{ "type": "message", "body": "Привет", "related_to": 123 }
```

Server message:

```json
{ "type": "message", "message": { "message_id": 15, "chat_id": 3, "author_id": 7, "author_name": "...", "author_tag": "...", "body": "Привет", "created_at": "..." } }
```

Error event:

```json
{ "type": "error", "error": "некорректное сообщение" }
```

Текущая реализация `Hub` находится в памяти одного процесса. Клиенты группируются по chat ID. Для каждого соединения есть одна write goroutine, очередь отправки, read pump, ping/pong, deadline и лимит frame size. Медленный клиент отключается, чтобы не блокировать рассылку остальным. Доступ к чату проверяется до Upgrade. При shutdown `Hub.CloseAll` закрывает соединения и ждёт завершения handler goroutines до закрытия DB pool.

**Ограничение масштабирования:** сообщения рассылаются только внутри одного экземпляра `chat_service`. При нескольких репликах понадобится межпроцессный pub/sub (например, Redis/NATS/PostgreSQL NOTIFY — выбор должен учитывать реальные требования). Не утверждайте, что текущий Hub распределённый.

## 9. Изображения, pins и desks

`pin_service` отвечает за pin/desk CRUD, связи пинов с досками, likes и serving изображений. `pkg/images` отвечает за файловое хранилище, проверки изображений, безопасные имена и публичные URL. Аватары профиля/досок и картинки пинов загружаются через multipart; клиент не должен подменять сохранённый avatar URL произвольным внешним URL.

`templates/pin1.jpg`–`pin30.jpg` используются seed migration `005_insert.sql`. Compose service `image_storage_init` копирует demo images в volume `pin_images`; `pin_service` и `user_service` монтируют этот volume. `/images/{filename}` обслуживается image handler.

## 10. HTTP API — текущие основные маршруты

Маршруты регистрируются в `auth_service/app.go`, `user_service/app.go`, `pin_service/app.go`, `chat_service/app.go`.

### Auth

- `POST /api/v1/auth/register`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/logout` (auth + CSRF)

### User profile

- `GET /api/v1/users/me`
- `PATCH /api/v1/users/me` (auth + CSRF)
- `DELETE /api/v1/users/me` (auth + CSRF)

### Pins, desks, likes, images

- `POST /api/v1/pins/search`
- `GET /api/v1/pins/{pinID}`
- `POST /api/v1/pins` (auth + CSRF)
- `PATCH /api/v1/pins/{pinID}` (auth + CSRF)
- `DELETE /api/v1/pins/{pinID}` (auth + CSRF)
- `POST /api/v1/desks` (auth + CSRF)
- `GET /api/v1/desks/me` (auth)
- `GET /api/v1/users/{userID}/desks`
- `GET /api/v1/desks/{deskID}` (optional auth)
- `PATCH /api/v1/desks/{deskID}` (auth + CSRF)
- `DELETE /api/v1/desks/{deskID}` (auth + CSRF)
- `POST /api/v1/desks/{deskID}/pins/{pinID}` (auth + CSRF)
- `DELETE /api/v1/desks/{deskID}/pins/{pinID}` (auth + CSRF)
- `POST /api/v1/pins/{pinID}/like` (auth + CSRF)
- `DELETE /api/v1/pins/{pinID}/like` (auth + CSRF)
- `GET /api/v1/pins/{pinID}/likes` (optional auth)
- `GET /api/v1/desks/{deskID}/pins/preview` (optional auth)
- `GET /images/{filename}`

При изменении маршрутов синхронизируйте Swagger annotations и OpenAPI workflow/docs, если endpoint должен быть задокументирован.

## 11. Тестирование и инструменты

Тесты находятся рядом с пакетами (`*_test.go`). Предпочтительные уровни:

- domain/usecase unit tests — ошибки, граничные значения, нормализация, отсутствие вызовов repository при invalid input;
- delivery/http tests — `httptest`, валидные/невалидные path/query/body, HTTP status и JSON contract;
- repository tests — `pgxmock` для SQL args/rows/errors; запросы должны оставаться parameterized;
- middleware tests — headers, context, JWT, CSRF, CORS, request id, SQL tracer;
- WebSocket tests — `httptest.Server`/gorilla client, несколько клиентов, broadcast, invalid frames, shutdown.

При новом transport validation тестируйте не только статус 400, но и то, что mock service/use-case **не был вызван**. Для pagination проверяйте lower/upper boundaries и дублирующиеся query values. Для path IDs проверяйте non-numeric, zero, negative и overflow.

Команды из корня проекта:

```bash
gofmt -w ./cmd ./pkg ./auth_service ./pin_service ./user_service ./chat_service
go test ./...
go test -cover ./...
go test -race ./...
go build ./cmd/...
make cover
make generate-mocks
make swagger
make run
# или
docker compose up --build
docker compose down
```

`make cover` создаёт coverage profile, исключает generated mocks и завершает работу с ошибкой при total coverage `<= 60%`. Обычный `go test -cover ./...` включает статистику всех пакетов, в том числе mocks, и сам по себе не применяет этот порог.

В `go.mod` указано Go `1.27`; для воспроизводимой проверки используйте эту версию или более новую совместимую. Не понижайте `go` directive только ради локальной старой среды.

## 12. OpenAPI, mocks и Docker Compose

- `.github/workflows/openapi.yml` выполняет генерацию/публикацию OpenAPI artifacts.
- `Makefile` содержит `swagger-auth`, `swagger-pin`, `swagger-user`, `swagger-chat` и агрегирующий `swagger`.
- `make generate-mocks` перегенерирует gomock mocks из интерфейсов. После изменения интерфейса проверьте, какие mocks устарели, и обновите их.
- `Dockerfile` собирает бинарник выбранного сервиса с build arg `SERVICE` и Go 1.27 Alpine.
- `Dockerfile.migrate` ставит Tern и применяет `db/migrations`.
- `docker-compose.yaml` поднимает PostgreSQL, миграционный job, image init job и четыре backend service containers. Сервисы ожидают готовность миграции.

Generated files не редактируйте вручную, если они могут быть пересобраны workflow/Makefile. После изменения публичных DTO/маршрутов проверьте annotations и повторите генерацию соответствующей документации.

## 13. Зафиксированные архитектурные решения и предыдущие изменения

При дальнейшей работе сохраняйте следующие текущие решения:

1. **Единый PostgreSQL setup.** У service package нет собственных `SetupPostgres`; пул создаётся в `server.Run` через `database.NewPostgresPool`.
2. **Единый Request ID middleware.** `middleware.RequestLogger` — единственный HTTP механизм выдачи/проброса ID и scoped logger.
3. **Request ID для WS операций.** Handshake и отдельные входящие сообщения имеют разные scope; у каждого сообщения собственный UUID, связанный с use-case и SQL logs.
4. **Request-scoped logging.** В HTTP/WS обработке использовать context logger; SQL args не логировать.
5. **Чат как отдельный сервис.** HTTP routes, use-cases, PostgreSQL repository и in-memory Hub разделены по слоям.
6. **Нормализованное хранение сообщений.** `chat_message` хранит `author_id`; имя и user tag читаются через JOIN и попадают в `MessageView`/`MessageResponse`, а не копируются в каждой строке.
7. **Message entity и response DTO разделены.** Не сериализуйте внутреннюю persistent model напрямую, если API contract требует read-time profile fields.
8. **Валидация в двух слоях.** HTTP transport проверяет формат/диапазон path, query и body; use-case повторяет бизнес-валидацию.
9. **Безопасное закрытие WS.** Перед закрытием DB pool нужно завершить WebSocket handlers.
10. **Миграции Tern.** Изменения схемы добавляются новыми последовательными migrations.

## 14. Инструкции AI-агенту для будущих изменений

Перед внесением изменений:

1. Прочитайте этот файл, `README.md`, `go.mod` и затрагиваемые package files.
2. Найдите все места создания/использования сущности, интерфейса, DTO, маршрута и migration, а не только первый файл из задачи.
3. Проследите полный путь данных: HTTP/WS input -> validation -> usecase -> repository -> DB -> read model -> response DTO.
4. Проверьте, как похожая операция уже сделана в соседнем service/handler; соблюдайте локальный стиль.
5. Не предполагайте, что текст этого guide гарантированно свежее кода. Если есть расхождение, изучите код и тесты и затем обновите guide.

При реализации:

- делайте минимальные согласованные изменения, не устраивайте несвязанный массовый рефакторинг;
- оставляйте бизнес-правила в domain/usecase, transport parsing и validation — в delivery;
- передавайте context до repository/pgx;
- используйте параметризованные SQL-запросы;
- используйте `middleware.LoggerFromContext(ctx)` в request processing;
- не логируйте пароли, токены, SQL bind args, полные пользовательские сообщения и другие секретные/персональные данные;
- не возвращайте pgx errors напрямую клиенту; переводите известные domain errors в ожидаемые HTTP statuses;
- не ломайте JSON names, cookie names, headers, route patterns и WebSocket envelope без явного требования;
- не храните вычисляемые user profile fields в каждой записи сообщения без причины;
- добавляйте unit, handler, repository и regression tests на соответствующих слоях;
- обновляйте mocks, Swagger/OpenAPI и README/этот guide, если изменилось поведение или контракт.

В финальном отчёте агент должен явно разделять: что изменено, какие проверки реально выполнены, какие не удалось запустить и почему. Не заявляйте об успешном `go test`/build без фактического выполнения.
