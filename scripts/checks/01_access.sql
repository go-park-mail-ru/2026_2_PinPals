\set ON_ERROR_STOP off
\pset pager off

\echo '=== Матрица прав на таблицы (из information_schema) ==='
SELECT grantee, table_name,
       string_agg(privilege_type, ',' ORDER BY privilege_type) AS privileges
FROM information_schema.table_privileges
WHERE table_schema = 'public'
  AND grantee IN ('auth_svc','user_svc','pin_svc','admin_ro')
GROUP BY grantee, table_name
ORDER BY grantee, table_name;

\echo
\echo '=== auth_svc ==='
SET ROLE auth_svc;
\echo '[OK]     читает "password"';             SELECT count(*) FROM "password";
\echo '[DENIED] читает pin';                    SELECT count(*) FROM pin;
\echo '[DENIED] удаляет пользователей';         DELETE FROM "user" WHERE false;
RESET ROLE;

\echo
\echo '=== user_svc ==='
SET ROLE user_svc;
\echo '[OK]     меняет профиль';                UPDATE "user" SET description = description WHERE false;
\echo '[DENIED] меняет birth_date';             UPDATE "user" SET birth_date = '2000-01-01' WHERE false;
\echo '[DENIED] читает "password"';             SELECT password_hash FROM "password" LIMIT 1;
\echo '[DENIED] читает pin';                    SELECT count(*) FROM pin;
RESET ROLE;

\echo
\echo '=== pin_svc ==='
SET ROLE pin_svc;
\echo '[OK]     читает автора пина (тег, ник, аватар)'; SELECT user_id, name, user_tag, avatar_url FROM "user" LIMIT 1;
\echo '[DENIED] читает birth_date';             SELECT birth_date FROM "user" LIMIT 1;
\echo '[DENIED] читает "password"';             SELECT password_hash FROM "password" LIMIT 1;
\echo '[DENIED] читает чаты';                   SELECT count(*) FROM chat_message;
\echo '[DENIED] DROP TABLE';                    DROP TABLE pin;
RESET ROLE;

\echo
\echo '=== admin_ro ==='
SET ROLE admin_ro;
\echo '[OK]     читает метрики';                SELECT active_users_day, new_pins_day FROM v_metrics_overview;
\echo '[OK]     читает pg_stat_activity (pg_monitor)'; SELECT count(*) FROM pg_stat_activity;
\echo '[DENIED] читает "password"';             SELECT count(*) FROM "password";
\echo '[DENIED] пишет в pin';                   INSERT INTO pin (creator_id, image_url, name) VALUES (1, 'x', 'x');
RESET ROLE;
