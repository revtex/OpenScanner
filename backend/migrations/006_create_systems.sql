-- +migrate Up
CREATE TABLE IF NOT EXISTS systems (
    id                       INTEGER PRIMARY KEY AUTOINCREMENT,
    system_id                INTEGER UNIQUE NOT NULL,
    label                    TEXT    NOT NULL,
    auto_populate_talkgroups INTEGER NOT NULL DEFAULT 1,
    blacklists_json          TEXT,
    led                      TEXT,
    "order"                  INTEGER NOT NULL DEFAULT 0
);

-- +migrate Down
DROP TABLE IF EXISTS systems;
