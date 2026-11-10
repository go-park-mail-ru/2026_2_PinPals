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
3. PostgreSQL доступен сервисам внутри compose-сети на `postgres:5432`. Порты HTTP-сервисов на хосте: `8001` (auth), `8002` (pins), `8003` (users).

### Изображения

`templates/pin1.jpg`–`templates/pin30.jpg` — демонстрационные изображения, на которые ссылается seed-миграция `db/migrations/005_insert.sql`. Одноразовый `image_storage_init` копирует их в общий volume `pin_images`; `pin_service` раздаёт их и загруженные изображения через `/images/{filename}`. Аватары профиля и досок загружаются как файлы `multipart/form-data`, а не как переданные клиентом URL.

### OpenAPI

OpenAPI-файлы генерируются workflow `.github/workflows/openapi.yml` и доступны как артефакт GitHub Actions; публикация GitHub Pages выполняется при push в `develop`. Swagger UI не входит в runtime-маршруты сервисов.
