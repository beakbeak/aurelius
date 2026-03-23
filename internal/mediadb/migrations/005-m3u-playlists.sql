-- v5: M3U playlist tables.
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
