# Схема БД

```mermaid
erDiagram
    direction TB

    USER {
        int user_id PK
        text name
        text user_tag
        int age
        text description
        text avatar_url
        bool deleted
        timestamptz created_at
        timestamptz updated_at
        timestamptz deleted_at
    }

    USER_SESSION {
        int session_id PK
        int user_id FK
        timestamptz session_start
        timestamptz session_end
    }

    DESK {
        int desk_id PK
        int creator_id FK
        text name
        text description
        text avatar_img_url
        smallint entry_status
        bool deleted
        timestamptz created_at
        timestamptz updated_at
        timestamptz deleted_at
    }

    PIN {
        int pin_id PK
        int creator_id FK
        text image_url
        text name
        text description
        bool deleted
        timestamptz created_at
        timestamptz updated_at
        timestamptz deleted_at
    }

    COMMENTARY {
        int comment_id PK
        int author_id FK
        int post_id FK
        text body
        bool deleted
        timestamptz created_at
        timestamptz updated_at
        timestamptz deleted_at
    }

    PIN_LIKE {
        int pin_like_id PK
        int liker_id FK
        int post_id FK
        timestamptz created_at
    }

    COMMENT_LIKE {
        int comm_like_id PK
        int commenter_id FK
        int comm_id FK
        timestamptz created_at
    }

    SUBSCRIPTION {
        int subscription_id PK
        int subscribee_id FK
        int subscriber_id FK
        timestamptz created_at
    }

    CHAT {
        int chat_id PK
        int user_a_id FK
        int user_b_id FK
        text name
        bool deleted
        timestamptz created_at
        timestamptz updated_at
        timestamptz deleted_at
    }

    CHAT_MESSAGE {
        int message_id PK
        int chat_id FK
        int author_id FK
        text body
        int related_to FK
        bool deleted
        timestamptz created_at
        timestamptz updated_at
        timestamptz deleted_at
    }

    PIN_DESK_RELATION {
        int desk_relation_id PK
        int desk_id FK
        int pin_id FK
        timestamptz created_at
    }

    TAG {
        int tag_id PK
        text name
        timestamptz created_at
    }

    PIN_TAG_RELATION {
        int tag_relation_id PK
        int tag_id FK
        int pin_id FK
        timestamptz created_at
    }

    USER       ||--o{ USER_SESSION       : has
    USER       ||--o{ DESK               : creates
    USER       ||--o{ PIN                : creates
    USER       ||--o{ COMMENTARY         : writes
    USER       ||--o{ PIN_LIKE           : likes
    USER       ||--o{ COMMENT_LIKE       : likes
    USER       ||--o{ SUBSCRIPTION       : subscriber
    USER       ||--o{ SUBSCRIPTION       : subscribee
    USER       ||--o{ CHAT               : participant_a
    USER       ||--o{ CHAT               : participant_b
    USER       ||--o{ CHAT_MESSAGE       : author

    DESK       ||--o{ PIN_DESK_RELATION  : contains
    PIN        ||--o{ PIN_DESK_RELATION  : belongs_to
    PIN        ||--o{ COMMENTARY         : has
    PIN        ||--o{ PIN_LIKE           : has
    PIN        ||--o{ PIN_TAG_RELATION   : has

    COMMENTARY ||--o{ COMMENT_LIKE       : has

    CHAT       ||--o{ CHAT_MESSAGE       : contains
    CHAT_MESSAGE ||--o{ CHAT_MESSAGE     : reply_to

    TAG        ||--o{ PIN_TAG_RELATION   : tagged
```

## Дополнительные хранилища
Помимо postgreSQL, используется Redis и MinIO
### Redis
Redis — для хранения сессий(чаты и заход/выход пользователя)

#### Сессии
**`session:{session_id}`** — hash. Одна активная сессия пользователя.
- `user_id` (integer) — id пользователя из `Postgres.USER.user_id`.
- `ip` (string) — IP-адрес, с которого установлена сессия.
- `created_at` (timestamp) — момент логина. Показывает, когда началась сессия; нужен для аналитики длительности.
- `last_activity` (timestamp) — момент последнего действия. Обновляется при каждом запросе;

#### Чат
**`chat:online:{user_id}`** — string. тут хранится `last_activity` (timestamp последнего WebSocket-пинга или сообщения).
**`chat:typing:{chat_id}:{user_id}`** — string.
**`chat:unread:{user_id}:{chat_id}`** — string (integer) счётчик непрочитанных сообщений.

### MinIO / S3
MinIO — Для хранения Пинов и аватарок(в PostgreSQL для файлов есть только неудобный и слишком ёмкий формат blob)

#### Bucket `avatars`
Хранит аватары пользователей.
Связь с Postgres: `Postgres.USER.avatar_url` → URL или ключ в этом bucket'е.

#### Bucket `pins`
Хранит изображения пинов.
Связь с Postgres: `Postgres.PIN.image_url` → URL или ключ в этом bucket'е.

#### Bucket `desks`
Хранит обложки досок.
Связь с Postgres: `Postgres.DESK.avatar_img_url` → URL или ключ в этом bucket'е.
