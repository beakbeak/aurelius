-- v3: Favorites table.
CREATE TABLE favorites (
    track_id INTEGER PRIMARY KEY REFERENCES tracks_with_deletes(id) ON DELETE CASCADE,
    added_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);
