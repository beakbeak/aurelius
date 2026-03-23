-- v8: Refactor play_history_plus to use a CTE, avoiding expression duplication.
DROP VIEW play_history_plus;

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
FROM base;
