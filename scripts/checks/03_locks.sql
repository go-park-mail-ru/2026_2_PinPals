#!/usr/bin/env bash
# Проверка 3. Блокировки и дедлок.
set -u
P="psql -X -q -v ON_ERROR_STOP=0 -P pager=off"

echo "=== A. Ожидание строки: сессия A держит строку 8 с, сессия B (user_svc) ждёт ==="
$P -c "BEGIN; UPDATE \"user\" SET description = 'A holds' WHERE user_id = 1; SELECT pg_sleep(8); COMMIT;" &
sleep 1
# важно: подключаемся именно КАК user_svc (-U), а не через SET ROLE:
# настройки роли (lock_timeout, application_name) применяются только при входе
psql -X -q -U user_svc -c "UPDATE \"user\" SET description = 'B waits' WHERE user_id = 1;" 2>&1 | sed 's/^/B: /' &
sleep 2.5
echo "--- наблюдатель: кто кого блокирует"
$P <<'SQL'
SELECT a.pid, a.application_name AS app, a.state, a.wait_event_type, a.wait_event,
       pg_blocking_pids(a.pid) AS blocked_by, left(a.query, 60) AS query
FROM pg_stat_activity a
WHERE a.datname = current_database() AND a.pid <> pg_backend_pid()
  AND (a.wait_event_type = 'Lock' OR cardinality(pg_blocking_pids(a.pid)) > 0 OR a.state = 'active')
ORDER BY a.pid;
SELECT locktype, mode, granted, pid FROM pg_locks WHERE NOT granted;
SQL
wait
echo "(у user_svc lock_timeout=3s: B должен завершиться ошибкой \"lock timeout\", а не ждать бесконечно)"

echo
echo "=== B. Дедлок: две транзакции берут строки 1 и 2 в разном порядке ==="
$P -c "BEGIN; UPDATE \"user\" SET description = 'd1' WHERE user_id = 1; SELECT pg_sleep(2); UPDATE \"user\" SET description = 'd1' WHERE user_id = 2; COMMIT;" 2>&1 | sed 's/^/T1: /' &
sleep 0.5
$P -c "BEGIN; UPDATE \"user\" SET description = 'd2' WHERE user_id = 2; SELECT pg_sleep(2); UPDATE \"user\" SET description = 'd2' WHERE user_id = 1; COMMIT;" 2>&1 | sed 's/^/T2: /' &
wait
$P -c "SELECT pg_stat_clear_snapshot();" >/dev/null
$P -c "SELECT deadlocks FROM pg_stat_database WHERE datname = current_database();"
echo "Решение: в коде брать блокировки в едином порядке (например, по возрастанию id) и держать транзакции короткими."
