-- Run with psql -v user_id=<existing user id>. All subsequent grants use the admin API.
\set ON_ERROR_STOP on
BEGIN;
SELECT pg_advisory_xact_lock(780026);
SELECT set_config('onlineconf.bootstrap_user', :'user_id', true);
DO $$
DECLARE actor integer := current_setting('onlineconf.bootstrap_user')::integer;
BEGIN
    IF EXISTS (SELECT 1 FROM onlineconf.history WHERE action='bootstrap') THEN
        RAISE EXCEPTION 'OnlineConf already initialized';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM public."user" WHERE user_id=actor AND deleted_at IS NULL) THEN
        RAISE EXCEPTION 'Active user required';
    END IF;
    INSERT INTO onlineconf.user_role VALUES(actor,'owner');
    INSERT INTO onlineconf.history(kind,resource_id,key,scopes,action,actor_id,version,after_json)
    SELECT 'admins',actor::text,actor::text,scopes,'bootstrap',actor,1,jsonb_build_object('role','owner')
    FROM onlineconf.role WHERE name='owner';
END $$;
COMMIT;
