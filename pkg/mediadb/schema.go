package mediadb

// migrations is an ordered list of SQL migrations. Each entry corresponds to a
// database version. The database's PRAGMA user_version tracks which migrations
// have been applied.
var migrations = []string{
	`-- v1: initial schema.
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

CREATE INDEX idx_dirs_parent ON dirs(parent);`,

	`-- v2: soft-delete tracks.
ALTER TABLE tracks RENAME TO tracks_with_deletes;
ALTER TABLE tracks_with_deletes ADD COLUMN deleted INTEGER NOT NULL DEFAULT 0;
CREATE VIEW tracks AS
  SELECT id, dir, name, mtime, hash, tags, attached_images, metadata
  FROM tracks_with_deletes
  WHERE deleted = 0;`,

	`-- v3: favorites table.
CREATE TABLE favorites (
    track_id INTEGER PRIMARY KEY REFERENCES tracks_with_deletes(id) ON DELETE CASCADE,
    added_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);`,

	`-- v4: FTS5 path search index.
CREATE VIRTUAL TABLE path_search_index USING fts5(
    path,
    type UNINDEXED,
    tokenize='trigram case_sensitive 0 remove_diacritics 1'
);

-- Backfill from existing data.
INSERT INTO path_search_index (path, type)
SELECT CASE WHEN dir = '' THEN name ELSE dir || '/' || name END, 'track'
FROM tracks;

INSERT INTO path_search_index (path, type)
SELECT path, 'dir' FROM dirs;

-- Auto-sync triggers for tracks.
CREATE TRIGGER path_search_track_ai AFTER INSERT ON tracks_with_deletes
WHEN NEW.deleted = 0
BEGIN
    INSERT INTO path_search_index (path, type) VALUES (
        CASE WHEN NEW.dir = '' THEN NEW.name
             ELSE NEW.dir || '/' || NEW.name END,
        'track'
    );
END;

CREATE TRIGGER path_search_track_au AFTER UPDATE ON tracks_with_deletes
BEGIN
    DELETE FROM path_search_index
    WHERE OLD.deleted = 0
      AND type = 'track'
      AND path = CASE WHEN OLD.dir = '' THEN OLD.name
                      ELSE OLD.dir || '/' || OLD.name END;
    INSERT INTO path_search_index (path, type)
    SELECT CASE WHEN NEW.dir = '' THEN NEW.name
                ELSE NEW.dir || '/' || NEW.name END,
           'track'
    WHERE NEW.deleted = 0;
END;

CREATE TRIGGER path_search_track_ad AFTER DELETE ON tracks_with_deletes
BEGIN
    DELETE FROM path_search_index
    WHERE type = 'track'
      AND path = CASE WHEN OLD.dir = '' THEN OLD.name
                      ELSE OLD.dir || '/' || OLD.name END;
END;

-- Auto-sync triggers for dirs.
CREATE TRIGGER path_search_dir_ai AFTER INSERT ON dirs
BEGIN
    INSERT INTO path_search_index (path, type) VALUES (NEW.path, 'dir');
END;

CREATE TRIGGER path_search_dir_ad AFTER DELETE ON dirs
BEGIN
    DELETE FROM path_search_index WHERE path = OLD.path AND type = 'dir';
END;`,

	`-- v5: m3u playlist tables.
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
);`,
}

// ReplayGain holds the four combinations of ReplayGain mode and clipping
// prevention.
type ReplayGain struct {
	Track       float64 `json:"track"`
	Album       float64 `json:"album"`
	TrackNoclip float64 `json:"trackNoclip"`
	AlbumNoclip float64 `json:"albumNoclip"`
}

// TrackMetadata holds audio properties stored in the metadata JSON column.
type TrackMetadata struct {
	Duration     float64     `json:"duration"`
	BitRate      int         `json:"bitRate"`
	SampleRate   uint        `json:"sampleRate"`
	SampleFormat string      `json:"sampleFormat"`
	ReplayGain   *ReplayGain `json:"replayGain,omitempty"`
}

// AttachedImageInfo describes an image attached to or associated with a track.
type AttachedImageInfo struct {
	MimeType string `json:"mimeType"`
	Size     int    `json:"size"`
}

// Track represents a row in the tracks table.
type Track struct {
	ID             int64
	Dir            string
	Name           string
	Mtime          int64
	Hash           []byte
	Tags           map[string]string
	AttachedImages []AttachedImageInfo
	Metadata       TrackMetadata
}

// Dir represents a row in the dirs table.
type Dir struct {
	Path   string
	Parent string
}

// M3UPlaylist represents a row in the m3u_playlists table.
type M3UPlaylist struct {
	ID    int64
	Dir   string
	Name  string
	Mtime int64
}
