-- People, their passkeys, their sign-in sessions, and the invite links that
-- let them in. Tokens are stored only as sha256 hashes.

CREATE TABLE users (
    id         INTEGER PRIMARY KEY,
    handle     BLOB    NOT NULL UNIQUE, -- the passkey user handle: 32 random bytes
    name       TEXT    NOT NULL,
    is_admin   INTEGER NOT NULL DEFAULT 0,
    created_at TEXT    NOT NULL
);
CREATE UNIQUE INDEX users_name ON users (name COLLATE NOCASE);

CREATE TABLE credentials (
    id           BLOB    PRIMARY KEY, -- the passkey's credential id
    user_id      INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    data         BLOB    NOT NULL,    -- the webauthn credential as json
    created_at   TEXT    NOT NULL,
    last_used_at TEXT
);
CREATE INDEX credentials_user ON credentials (user_id);

CREATE TABLE sessions (
    token_hash   BLOB    PRIMARY KEY,
    user_id      INTEGER NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at   TEXT    NOT NULL,
    last_seen_at TEXT    NOT NULL,
    expires_at   TEXT    NOT NULL
);
CREATE INDEX sessions_user ON sessions (user_id);

CREATE TABLE invites (
    token_hash  BLOB    PRIMARY KEY,
    created_by  INTEGER REFERENCES users (id) ON DELETE SET NULL,
    for_user_id INTEGER REFERENCES users (id) ON DELETE CASCADE, -- a re-invite adds a passkey to this user
    is_admin    INTEGER NOT NULL DEFAULT 0,
    created_at  TEXT    NOT NULL,
    expires_at  TEXT    NOT NULL,
    used_at     TEXT,
    used_by     INTEGER REFERENCES users (id) ON DELETE SET NULL
);
