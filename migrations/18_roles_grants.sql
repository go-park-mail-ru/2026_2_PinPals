DO $$
DECLARE
    r record;
BEGIN
    FOR r IN
        SELECT * FROM (VALUES
            ('migrator',  5),
            ('auth_svc',  20),
            ('user_svc',  20),
            ('pin_svc',   30),
            ('admin_ro',  5)
        ) AS v(name, conn_limit)
    LOOP
        IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = r.name) THEN
            EXECUTE format(
                'CREATE ROLE %I LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION CONNECTION LIMIT %s',
                r.name, r.conn_limit);
        END IF;
    END LOOP;
END $$;

ALTER ROLE auth_svc SET application_name = 'auth-service';
ALTER ROLE user_svc SET application_name = 'user-service';
ALTER ROLE pin_svc  SET application_name = 'pin-service';
ALTER ROLE admin_ro SET application_name = 'admin-ro';

ALTER ROLE auth_svc SET statement_timeout = '5s';
ALTER ROLE user_svc SET statement_timeout = '5s';
ALTER ROLE pin_svc  SET statement_timeout = '5s';
ALTER ROLE auth_svc SET lock_timeout = '3s';
ALTER ROLE user_svc SET lock_timeout = '3s';
ALTER ROLE pin_svc  SET lock_timeout = '3s';
ALTER ROLE auth_svc SET idle_in_transaction_session_timeout = '10s';
ALTER ROLE user_svc SET idle_in_transaction_session_timeout = '10s';
ALTER ROLE pin_svc  SET idle_in_transaction_session_timeout = '10s';
ALTER ROLE admin_ro SET statement_timeout = '30s';
ALTER ROLE admin_ro SET default_transaction_read_only = on;

GRANT pg_monitor TO admin_ro;

DO $$
DECLARE
    r record;
BEGIN
    FOR r IN SELECT tablename FROM pg_tables WHERE schemaname = 'public' LOOP
        EXECUTE format('ALTER TABLE public.%I OWNER TO migrator', r.tablename);
    END LOOP;

    FOR r IN
        SELECT p.oid::regprocedure AS sig
        FROM pg_proc AS p
        WHERE p.pronamespace = 'public'::regnamespace
          AND NOT EXISTS (
              SELECT 1 FROM pg_depend AS d
              WHERE d.objid = p.oid AND d.deptype = 'e'
          )
    LOOP
        EXECUTE format('ALTER FUNCTION %s OWNER TO migrator', r.sig);
    END LOOP;
END $$;

REVOKE ALL ON SCHEMA public FROM PUBLIC;
GRANT USAGE         ON SCHEMA public TO auth_svc, user_svc, pin_svc, admin_ro;
GRANT USAGE, CREATE ON SCHEMA public TO migrator;

DO $$
BEGIN
    EXECUTE format('REVOKE ALL ON DATABASE %I FROM PUBLIC', current_database());
    EXECUTE format(
        'GRANT CONNECT ON DATABASE %I TO migrator, auth_svc, user_svc, pin_svc, admin_ro',
        current_database());
END $$;

GRANT SELECT, INSERT         ON "user"        TO auth_svc;
GRANT SELECT, INSERT, UPDATE ON "password"    TO auth_svc;
GRANT INSERT                 ON user_session  TO auth_svc;

GRANT SELECT ON "user" TO user_svc;
GRANT UPDATE (name, user_tag, description, avatar_url, updated_at, deleted_at)
    ON "user" TO user_svc;
GRANT SELECT, INSERT, DELETE ON subscription  TO user_svc;
GRANT SELECT, INSERT, UPDATE ON chat          TO user_svc;
GRANT SELECT, INSERT, UPDATE ON chat_message  TO user_svc;
GRANT SELECT                 ON user_session  TO user_svc;

GRANT SELECT (user_id, name, user_tag, avatar_url, deleted_at) ON "user" TO pin_svc;
GRANT SELECT, INSERT, UPDATE ON pin         TO pin_svc;  -- удаление мягкое (deleted_at)
GRANT SELECT, INSERT, UPDATE ON desk        TO pin_svc;
GRANT SELECT, INSERT, UPDATE ON commentary  TO pin_svc;
GRANT SELECT, INSERT         ON tag         TO pin_svc;
GRANT UPDATE (name)          ON tag         TO pin_svc;
GRANT SELECT, INSERT, DELETE ON pin_like          TO pin_svc;
GRANT SELECT, INSERT, DELETE ON comment_like      TO pin_svc;
GRANT SELECT, INSERT, DELETE ON pin_desk_relation TO pin_svc;
GRANT SELECT, INSERT, DELETE ON pin_tag_relation  TO pin_svc;

GRANT SELECT ON
    "user", pin, desk, commentary, pin_like, comment_like, subscription,
    pin_desk_relation, tag, pin_tag_relation, chat, chat_message, user_session
TO admin_ro;


ALTER DEFAULT PRIVILEGES FOR ROLE migrator IN SCHEMA public
    GRANT SELECT ON TABLES TO admin_ro;
