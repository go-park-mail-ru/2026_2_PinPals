-- Write your migrate up statements here
CREATE TABLE revoked_jwt_token (
    token_id   text        PRIMARY KEY,
    expires_at timestamptz NOT NULL
);

CREATE INDEX revoked_jwt_token_expires_at_idx ON revoked_jwt_token (expires_at);

---- create above / drop below ----

-- Write your migrate down statements here.

DROP TABLE IF EXISTS revoked_jwt_token;
