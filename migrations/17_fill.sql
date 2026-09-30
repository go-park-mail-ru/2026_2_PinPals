WITH inserted_user AS (
    INSERT INTO "user" (name, user_tag, age, description, avatar_url)
    VALUES
        ('Alice',   'alice',   25, 'Photographer and traveler', 'https://minio.local/avatars/alice.png'),
        ('Bob',     'bob',     30, 'UI/UX designer',            'https://minio.local/avatars/bob.png'),
        ('Charlie', 'charlie', 22, NULL,                        NULL),
        ('Diana',   'diana',   28, 'Illustrator',               'https://minio.local/avatars/diana.png')
    RETURNING user_id, user_tag
),
alice   AS (SELECT user_id FROM inserted_user WHERE user_tag = 'alice'),
bob     AS (SELECT user_id FROM inserted_user WHERE user_tag = 'bob'),
diana   AS (SELECT user_id FROM inserted_user WHERE user_tag = 'diana'),

inserted_desk AS (
    INSERT INTO desk (creator_id, name, description, entry_status)
    VALUES
        ((SELECT user_id FROM alice), 'Travel', 'Places I want to visit', 0),
        ((SELECT user_id FROM bob),   'Design', 'UI inspiration',         0),
        ((SELECT user_id FROM diana), 'Secret', NULL,                     1)
    RETURNING desk_id, name
)
