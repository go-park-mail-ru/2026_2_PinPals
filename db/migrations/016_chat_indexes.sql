-- Write your migrate up statements here
CREATE INDEX IF NOT EXISTS idx_chat_user_a_updated_at
    ON chat (user_a_id, updated_at DESC)
    WHERE deleted = false;

CREATE INDEX IF NOT EXISTS idx_chat_user_b_updated_at
    ON chat (user_b_id, updated_at DESC)
    WHERE deleted = false;

CREATE INDEX IF NOT EXISTS idx_chat_message_chat_message_id
    ON chat_message (chat_id, message_id DESC)
    WHERE deleted = false;

---- create above / drop below ----

-- Write your migrate down statements here.
DROP INDEX IF EXISTS idx_chat_message_chat_message_id;
DROP INDEX IF EXISTS idx_chat_user_b_updated_at;
DROP INDEX IF EXISTS idx_chat_user_a_updated_at;
