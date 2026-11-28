CREATE EXTENSION IF NOT EXISTS pgcrypto;

WITH admin_user AS (
    INSERT INTO public."user" (
        name,
        user_tag,
        birth_date,
        description,
        avatar_url
    )
    VALUES (
        'admin34',
        'admin34',
        DATE '1990-01-01',
        'System administrator',
        NULL
    )
    ON CONFLICT (user_tag) DO UPDATE
    SET
        name = EXCLUDED.name,
        deleted_at = NULL
    RETURNING user_id
)

INSERT INTO public."password" (
    user_id,
    password_hash
)
SELECT
    user_id,
    crypt('wizard13570', gen_salt('bf', 10))
FROM admin_user
ON CONFLICT (user_id) DO UPDATE
SET
    password_hash = EXCLUDED.password_hash,
    updated_at = now();

INSERT INTO onlineconf.user_role (
    user_id,
    role
)
SELECT
    user_id,
    'owner'
FROM public."user"
WHERE user_tag = 'admin34'
  AND deleted_at IS NULL
ON CONFLICT (user_id, role) DO NOTHING;

INSERT INTO onlineconf.history (
    kind,
    resource_id,
    key,
    scopes,
    action,
    actor_id,
    after_json,
    version,
    comment
)
SELECT
    'admins',
    u.user_id::text,
    u.user_tag,
    r.scopes,
    CASE
        WHEN EXISTS (
            SELECT 1
            FROM onlineconf.history
            WHERE action = 'bootstrap'
        )
        THEN 'admin_seed'
        ELSE 'bootstrap'
    END,
    u.user_id,
    jsonb_build_object('role', 'owner'),
    1,
    'Initial OnlineConf administrator'
FROM public."user" u
JOIN onlineconf.role r ON r.name = 'owner'
WHERE u.user_tag = 'admin34'
  AND u.deleted_at IS NULL;
