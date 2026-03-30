CREATE TABLE IF NOT EXISTS "tracks_with_deletes" (
    id              INTEGER PRIMARY KEY,
    dir             TEXT NOT NULL,
    name            TEXT NOT NULL,
    mtime           INTEGER NOT NULL,
    hash            BLOB NOT NULL,
    tags            TEXT NOT NULL DEFAULT '{}',
    metadata        TEXT NOT NULL DEFAULT '{}', deleted INTEGER NOT NULL DEFAULT 0,

    UNIQUE(dir, name)
);
CREATE INDEX idx_tracks_dir_name ON "tracks_with_deletes"(dir, name);
CREATE TABLE dirs (
    path    TEXT PRIMARY KEY,
    parent  TEXT NOT NULL
, image_fingerprint BLOB);
CREATE INDEX idx_dirs_parent ON dirs(parent);
CREATE TABLE favorites (
    track_id INTEGER PRIMARY KEY REFERENCES tracks_with_deletes(id) ON DELETE CASCADE,
    added_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE TABLE m3u_playlists (
    id    INTEGER PRIMARY KEY,
    dir   TEXT NOT NULL,
    name  TEXT NOT NULL,
    mtime INTEGER NOT NULL,

    UNIQUE(dir, name)
);
CREATE INDEX idx_m3u_playlists_dir ON m3u_playlists(dir);
CREATE TABLE m3u_playlist_tracks (
    playlist_id INTEGER NOT NULL REFERENCES m3u_playlists(id) ON DELETE CASCADE,
    position    INTEGER NOT NULL,
    track_id    INTEGER NOT NULL REFERENCES tracks_with_deletes(id) ON DELETE CASCADE,

    PRIMARY KEY (playlist_id, position)
);
CREATE TABLE images (
    hash          BLOB PRIMARY KEY,
    original_hash BLOB NOT NULL,
    mime_type     TEXT NOT NULL,
    data          BLOB NOT NULL
, width INTEGER NOT NULL DEFAULT 0, height INTEGER NOT NULL DEFAULT 0);
CREATE INDEX idx_images_original_hash ON images(original_hash);
CREATE TABLE track_images (
    track_id   INTEGER NOT NULL REFERENCES tracks_with_deletes(id) ON DELETE CASCADE,
    position   INTEGER NOT NULL,
    image_hash BLOB NOT NULL REFERENCES images(hash),

    PRIMARY KEY (track_id, position)
);
CREATE VIEW tracks AS
  SELECT id, dir, name, mtime, hash, tags, metadata
  FROM tracks_with_deletes
  WHERE deleted = 0
/* tracks(id,dir,name,mtime,hash,tags,metadata) */;
CREATE TABLE play_history (
    id        INTEGER PRIMARY KEY,
    track_id  INTEGER NOT NULL REFERENCES tracks_with_deletes(id) ON DELETE CASCADE,
    played_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
CREATE INDEX idx_play_history_played_at ON play_history(played_at);
CREATE VIEW play_history_plus AS
WITH base AS (
    SELECT
        ph.id,
        ph.track_id,
        ph.played_at,
        json_extract(t.metadata, '$.duration') AS duration,
        (unixepoch(LEAD(ph.played_at) OVER (ORDER BY ph.played_at))
            - unixepoch(ph.played_at)) AS seconds_played
    FROM play_history ph
    JOIN tracks_with_deletes t ON ph.track_id = t.id
)
SELECT
    *,
    CASE
        WHEN seconds_played IS NULL THEN 0
        WHEN seconds_played < (duration * 0.9) THEN 1
        ELSE 0
    END AS is_skipped
FROM base
/* play_history_plus(id,track_id,played_at,duration,seconds_played,is_skipped) */;
CREATE VIRTUAL TABLE search_index USING fts5(
    text,
    dir UNINDEXED,
    name UNINDEXED,
    type UNINDEXED,
    tokenize='trigram case_sensitive 0 remove_diacritics 1'
)
/* search_index(text,dir,name,type) */;
CREATE TABLE IF NOT EXISTS 'search_index_data'(id INTEGER PRIMARY KEY, block BLOB);
CREATE TABLE IF NOT EXISTS 'search_index_idx'(segid, term, pgno, PRIMARY KEY(segid, term)) WITHOUT ROWID;
CREATE TABLE IF NOT EXISTS 'search_index_content'(id INTEGER PRIMARY KEY, c0, c1, c2, c3);
CREATE TABLE IF NOT EXISTS 'search_index_docsize'(id INTEGER PRIMARY KEY, sz BLOB);
CREATE TABLE IF NOT EXISTS 'search_index_config'(k PRIMARY KEY, v) WITHOUT ROWID;
CREATE TRIGGER search_track_ai AFTER INSERT ON tracks_with_deletes
WHEN NEW.deleted = 0
BEGIN
    INSERT INTO search_index (text, dir, name, type) VALUES (
        NEW.dir || ' ' || CASE
            WHEN json_extract(NEW.tags, '$.artist') IS NOT NULL
             AND json_extract(NEW.tags, '$.title') IS NOT NULL
            THEN json_extract(NEW.tags, '$.artist')
                 || ' ' || json_extract(NEW.tags, '$.title')
                 || ' ' || COALESCE(json_extract(NEW.tags, '$.album'), '')
            ELSE NEW.name
        END,
        NEW.dir,
        NEW.name,
        'track'
    );
END;
CREATE TRIGGER search_track_au AFTER UPDATE ON tracks_with_deletes
WHEN OLD.dir != NEW.dir OR OLD.name != NEW.name
  OR OLD.deleted != NEW.deleted OR OLD.tags != NEW.tags
BEGIN
    DELETE FROM search_index
    WHERE OLD.deleted = 0
      AND type = 'track'
      AND dir = OLD.dir
      AND name = OLD.name;
    INSERT INTO search_index (text, dir, name, type)
    SELECT
        NEW.dir || ' ' || CASE
            WHEN json_extract(NEW.tags, '$.artist') IS NOT NULL
             AND json_extract(NEW.tags, '$.title') IS NOT NULL
            THEN json_extract(NEW.tags, '$.artist')
                 || ' ' || json_extract(NEW.tags, '$.title')
                 || ' ' || COALESCE(json_extract(NEW.tags, '$.album'), '')
            ELSE NEW.name
        END,
        NEW.dir,
        NEW.name,
        'track'
    WHERE NEW.deleted = 0;
END;
CREATE TRIGGER search_track_ad AFTER DELETE ON tracks_with_deletes
BEGIN
    DELETE FROM search_index
    WHERE type = 'track'
      AND dir = OLD.dir
      AND name = OLD.name;
END;
CREATE TRIGGER search_dir_ai AFTER INSERT ON dirs
BEGIN
    INSERT INTO search_index (text, dir, name, type) VALUES (NEW.path, NEW.path, '', 'dir');
END;
CREATE TRIGGER search_dir_ad AFTER DELETE ON dirs
BEGIN
    DELETE FROM search_index WHERE dir = OLD.path AND name = '' AND type = 'dir';
END;
