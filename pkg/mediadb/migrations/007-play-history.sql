-- v7: Play history table and skip-detection view.
CREATE TABLE play_history (
    id        INTEGER PRIMARY KEY,
    track_id  INTEGER NOT NULL REFERENCES tracks_with_deletes(id) ON DELETE CASCADE,
    played_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE INDEX idx_play_history_played_at ON play_history(played_at);

CREATE VIEW play_history_plus AS
SELECT
    ph.id,
    ph.track_id,
    ph.played_at,
    json_extract(t.metadata, '$.duration') AS duration,
    (unixepoch(LEAD(ph.played_at) OVER (ORDER BY ph.played_at))
        - unixepoch(ph.played_at)) AS seconds_played,
    CASE
        WHEN LEAD(ph.played_at) OVER (ORDER BY ph.played_at) IS NULL THEN 0
        WHEN (unixepoch(LEAD(ph.played_at) OVER (ORDER BY ph.played_at))
            - unixepoch(ph.played_at))
            < (json_extract(t.metadata, '$.duration') * 0.9) THEN 1
        ELSE 0
    END AS is_skipped
FROM play_history ph
JOIN tracks_with_deletes t ON ph.track_id = t.id;
