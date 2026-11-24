\set ON_ERROR_STOP off
\pset pager off

\echo '=== Откаты ДО ==='
SELECT xact_rollback, deadlocks FROM pg_stat_database WHERE datname = current_database();

\echo '--- 1. нарушение UNIQUE (user_tag)'
INSERT INTO "user" (name, user_tag, birth_date) VALUES ('Dup', 'alice', '2000-01-01');
\echo '--- 2. триггер check_user_age (младше 12 лет)'
INSERT INTO "user" (name, user_tag, birth_date) VALUES ('Kid', 'kid_test', current_date);
\echo '--- 3. нарушение FK (несуществующий автор пина)'
INSERT INTO pin (creator_id, image_url, name) VALUES (999999, 'https://x/fk', 'fk');
\echo '--- 4. CHECK: пустой отзыв'
INSERT INTO feedback (author_id, body) VALUES (1, '   ');
\echo '--- 5. permission denied (сервис ходит не в свою таблицу)'
SET ROLE auth_svc;  SELECT count(*) FROM pin;  RESET ROLE;

\echo '=== Откаты ПОСЛЕ ==='
SELECT pg_sleep(1.5);
\connect
SELECT xact_rollback, deadlocks FROM pg_stat_database WHERE datname = current_database();
