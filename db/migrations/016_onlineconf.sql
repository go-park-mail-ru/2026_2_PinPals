CREATE SCHEMA onlineconf;

CREATE TABLE onlineconf.resource (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind text NOT NULL CHECK (kind IN ('config','flag')),
    key text UNIQUE NOT NULL,
    title text NOT NULL,
    description text NOT NULL DEFAULT '',
    scopes text[] NOT NULL,
    value_type text NOT NULL DEFAULT '',
    value jsonb,
    visibility text NOT NULL DEFAULT 'internal',
    active boolean NOT NULL DEFAULT true,
    rollout_percent integer NOT NULL DEFAULT 0 CHECK (rollout_percent BETWEEN 0 AND 100),
    archived boolean NOT NULL DEFAULT false,
    version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    created_by integer REFERENCES public."user"(user_id),
    updated_by integer REFERENCES public."user"(user_id)
);
CREATE INDEX resource_scope ON onlineconf.resource USING gin(scopes);
CREATE INDEX resource_title ON onlineconf.resource(lower(title));
CREATE INDEX resource_state ON onlineconf.resource(kind, active, archived, value_type);

CREATE TABLE onlineconf.override (
    flag_id bigint REFERENCES onlineconf.resource,
    user_id integer REFERENCES public."user"(user_id),
    included boolean NOT NULL,
    actor_id integer NOT NULL REFERENCES public."user"(user_id),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(flag_id,user_id)
);
CREATE INDEX override_user ON onlineconf.override(user_id);

CREATE TABLE onlineconf.plan (
    flag_id bigint PRIMARY KEY REFERENCES onlineconf.resource,
    name text NOT NULL,
    stages integer[] NOT NULL,
    stage integer NOT NULL,
    duration_seconds integer NOT NULL CHECK (duration_seconds >= 0),
    approver_role text NOT NULL DEFAULT '',
    status text NOT NULL CHECK (status IN ('running','paused','stopped','complete')),
    started_at timestamptz NOT NULL DEFAULT now()
);

CREATE VIEW onlineconf.details AS
SELECT r.*, ARRAY(SELECT user_id FROM onlineconf.override WHERE flag_id=r.id AND included ORDER BY user_id) AS allowlist,
       ARRAY(SELECT user_id FROM onlineconf.override WHERE flag_id=r.id AND NOT included ORDER BY user_id) AS denylist,
       (SELECT (to_jsonb(p)-'flag_id') || jsonb_build_object('approval_state',
            CASE WHEN p.status <> 'running' THEN p.status
                 WHEN now() >= p.started_at + make_interval(secs => p.duration_seconds) THEN 'awaiting_approval'
                 ELSE 'holding' END) FROM onlineconf.plan p WHERE p.flag_id=r.id) AS plan
FROM onlineconf.resource r;

CREATE TABLE onlineconf.history (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    kind text NOT NULL, resource_id text NOT NULL, key text NOT NULL,
    scopes text[] NOT NULL, action text NOT NULL, actor_id integer,
    created_at timestamptz NOT NULL DEFAULT now(),
    before_json jsonb, after_json jsonb, version bigint NOT NULL,
    comment text NOT NULL DEFAULT ''
);
CREATE INDEX history_time ON onlineconf.history(created_at);
CREATE INDEX history_resource ON onlineconf.history(kind,resource_id);
CREATE INDEX history_actor ON onlineconf.history(actor_id);

CREATE TABLE onlineconf.role (
    name text PRIMARY KEY, permissions text[] NOT NULL, scopes text[] NOT NULL
);
CREATE TABLE onlineconf.admin_group (name text PRIMARY KEY);
CREATE TABLE onlineconf.user_role (
    user_id integer REFERENCES public."user"(user_id), role text REFERENCES onlineconf.role ON DELETE CASCADE,
    PRIMARY KEY(user_id,role)
);
CREATE TABLE onlineconf.group_role (
    group_name text REFERENCES onlineconf.admin_group ON DELETE CASCADE, role text REFERENCES onlineconf.role ON DELETE CASCADE,
    PRIMARY KEY(group_name,role)
);
CREATE TABLE onlineconf.member (
    user_id integer REFERENCES public."user"(user_id), group_name text REFERENCES onlineconf.admin_group ON DELETE CASCADE,
    PRIMARY KEY(user_id,group_name)
);
CREATE VIEW onlineconf.effective_roles AS
SELECT user_id,role FROM onlineconf.user_role UNION
SELECT m.user_id,g.role FROM onlineconf.member m JOIN onlineconf.group_role g USING(group_name);

INSERT INTO onlineconf.role VALUES ('owner',
    ARRAY['config.read','config.create','config.update','config.delete','flag.read','flag.create','flag.update','flag.delete',
          'flag.evaluate','rollout.create','rollout.update','rollout.approve','rollout.pause','rollout.stop','rollout.emergency',
          'history.read','roles.manage','groups.manage','admins.manage'],
    ARRAY['frontend','auth_service','pin_service','user_service']);
INSERT INTO onlineconf.resource(kind,key,title,scopes,value_type,value,visibility)
VALUES ('config','pins.pagination.default_page_size','Размер страницы ленты',ARRAY['frontend','pin_service'],'integer','30','public');
INSERT INTO onlineconf.resource(kind,key,title,scopes,rollout_percent)
VALUES ('flag','pins.likes.enabled','Лайки пинов',ARRAY['frontend','pin_service'],100);
INSERT INTO onlineconf.history(kind,resource_id,key,scopes,action,after_json,version)
SELECT kind,id::text,key,scopes,'seed',to_jsonb(d),version FROM onlineconf.details d;

---- create above / drop below ----

DROP SCHEMA onlineconf CASCADE;
