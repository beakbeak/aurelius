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
