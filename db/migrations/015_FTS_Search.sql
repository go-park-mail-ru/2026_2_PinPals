-- Write your migrate up statements here

-- Конфигурация 'russian' ДОЛЖНА совпадать с websearch_to_tsquery('russian', ...) в запросах.
-- Для тегов пользователей стемминг не нужен — 'simple'.

ALTER TABLE pin
    ADD COLUMN name_tsv tsvector
    GENERATED ALWAYS AS (to_tsvector('russian', name)) STORED;
CREATE INDEX idx_pin_name_fts ON pin USING GIN (name_tsv) WHERE deleted_at IS NULL;

ALTER TABLE desk
    ADD COLUMN name_tsv tsvector
    GENERATED ALWAYS AS (to_tsvector('russian', name)) STORED;
CREATE INDEX idx_desk_name_fts ON desk USING GIN (name_tsv) WHERE deleted_at IS NULL;

ALTER TABLE "user"
    ADD COLUMN user_tag_tsv tsvector
    GENERATED ALWAYS AS (to_tsvector('simple', user_tag)) STORED;
CREATE INDEX idx_user_user_tag_fts ON "user" USING GIN (user_tag_tsv) WHERE deleted_at IS NULL;

---- create above / drop below ----

DROP INDEX IF EXISTS idx_user_user_tag_fts;
DROP INDEX IF EXISTS idx_desk_name_fts;
DROP INDEX IF EXISTS idx_pin_name_fts;

ALTER TABLE "user" DROP COLUMN IF EXISTS user_tag_tsv;
ALTER TABLE desk   DROP COLUMN IF EXISTS name_tsv;
ALTER TABLE pin    DROP COLUMN IF EXISTS name_tsv;
