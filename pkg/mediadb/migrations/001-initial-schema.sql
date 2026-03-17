-- v1: Initial schema.
CREATE TABLE tracks (
    id              INTEGER PRIMARY KEY,
    dir             TEXT NOT NULL,
    name            TEXT NOT NULL,
    mtime           INTEGER NOT NULL,
    hash            BLOB NOT NULL,
    tags            TEXT NOT NULL DEFAULT '{}',
    attached_images TEXT NOT NULL DEFAULT '[]',
    metadata        TEXT NOT NULL DEFAULT '{}',

    UNIQUE(dir, name)
);

CREATE INDEX idx_tracks_dir_name ON tracks(dir, name);

CREATE TABLE dirs (
    path    TEXT PRIMARY KEY,
    parent  TEXT NOT NULL
);

CREATE INDEX idx_dirs_parent ON dirs(parent);
