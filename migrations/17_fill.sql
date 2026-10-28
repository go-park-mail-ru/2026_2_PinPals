CREATE EXTENSION IF NOT EXISTS pgcrypto;

INSERT INTO "user"
    (name, user_tag, birth_date, description, avatar_url)
VALUES
    ('Alice', 'alice', DATE '2001-03-14',
     'Photographer and traveler', 'https://minio.local/avatars/alice.png'),
    ('Bob', 'bob', DATE '1996-03-01',
     'UI/UX designer', 'https://minio.local/avatars/bob.png'),
    ('Charlie', 'charlie', DATE '2004-09-01',
     NULL, NULL),
    ('Diana', 'diana', DATE '1998-04-10',
     'Illustrator', 'https://minio.local/avatars/diana.png'),
    ('Ультра Гигачелов', 'ultra_giga', DATE '2005-03-14',
     'Люблю мою крутую девушку', NULL)
ON CONFLICT (user_tag) DO NOTHING;

INSERT INTO "password" (user_id, password_hash)
SELECT
    user_id,
    crypt('qwerty123', gen_salt('bf', 10))
FROM "user"
WHERE user_tag = 'ultra_giga'
  AND deleted_at IS NULL
ON CONFLICT (user_id) DO NOTHING;

INSERT INTO desk (creator_id, name, description, entry_status)
SELECT
    u.user_id,
    seed.desk_name,
    seed.description,
    seed.entry_status
FROM (
    VALUES
        ('alice'::text, 'Travel'::text, 'Places I want to visit'::text, 0::smallint),
        ('bob'::text, 'Design'::text, 'UI inspiration'::text, 0::smallint),
        ('diana'::text, 'Secret'::text, NULL::text, 1::smallint)
) AS seed(user_tag, desk_name, description, entry_status)
JOIN "user" AS u
    ON u.user_tag = seed.user_tag
   AND u.deleted_at IS NULL
WHERE NOT EXISTS (
    SELECT 1
    FROM desk AS d
    WHERE d.creator_id = u.user_id
      AND d.name = seed.desk_name
      AND d.deleted_at IS NULL
);
