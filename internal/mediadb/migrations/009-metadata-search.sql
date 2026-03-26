-- v9: Index track metadata (artist, title, album) in FTS search.

-- Drop old triggers.
DROP TRIGGER path_search_track_ai;
DROP TRIGGER path_search_track_au;
DROP TRIGGER path_search_track_ad;
DROP TRIGGER path_search_dir_ai;
DROP TRIGGER path_search_dir_ad;

-- Drop old FTS table.
DROP TABLE path_search_index;

-- Create new FTS table with separate searchable text and library path.
CREATE VIRTUAL TABLE search_index USING fts5(
    text,
    path UNINDEXED,
    type UNINDEXED,
    tokenize='trigram case_sensitive 0 remove_diacritics 1'
);

-- Backfill tracks.
INSERT INTO search_index (text, path, type)
SELECT
    dir || ' ' || CASE
        WHEN json_extract(tags, '$.artist') IS NOT NULL
         AND json_extract(tags, '$.title') IS NOT NULL
        THEN json_extract(tags, '$.artist')
             || ' ' || json_extract(tags, '$.title')
             || ' ' || COALESCE(json_extract(tags, '$.album'), '')
        ELSE name
    END,
    CASE WHEN dir = '' THEN name ELSE dir || '/' || name END,
    'track'
FROM tracks;

-- Backfill dirs.
INSERT INTO search_index (text, path, type)
SELECT path, path, 'dir' FROM dirs;

-- Auto-sync triggers for tracks.
CREATE TRIGGER search_track_ai AFTER INSERT ON tracks_with_deletes
WHEN NEW.deleted = 0
BEGIN
    INSERT INTO search_index (text, path, type) VALUES (
        NEW.dir || ' ' || CASE
            WHEN json_extract(NEW.tags, '$.artist') IS NOT NULL
             AND json_extract(NEW.tags, '$.title') IS NOT NULL
            THEN json_extract(NEW.tags, '$.artist')
                 || ' ' || json_extract(NEW.tags, '$.title')
                 || ' ' || COALESCE(json_extract(NEW.tags, '$.album'), '')
            ELSE NEW.name
        END,
        CASE WHEN NEW.dir = '' THEN NEW.name
             ELSE NEW.dir || '/' || NEW.name END,
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
      AND path = CASE WHEN OLD.dir = '' THEN OLD.name
                      ELSE OLD.dir || '/' || OLD.name END;
    INSERT INTO search_index (text, path, type)
    SELECT
        NEW.dir || ' ' || CASE
            WHEN json_extract(NEW.tags, '$.artist') IS NOT NULL
             AND json_extract(NEW.tags, '$.title') IS NOT NULL
            THEN json_extract(NEW.tags, '$.artist')
                 || ' ' || json_extract(NEW.tags, '$.title')
                 || ' ' || COALESCE(json_extract(NEW.tags, '$.album'), '')
            ELSE NEW.name
        END,
        CASE WHEN NEW.dir = '' THEN NEW.name
             ELSE NEW.dir || '/' || NEW.name END,
        'track'
    WHERE NEW.deleted = 0;
END;

CREATE TRIGGER search_track_ad AFTER DELETE ON tracks_with_deletes
BEGIN
    DELETE FROM search_index
    WHERE type = 'track'
      AND path = CASE WHEN OLD.dir = '' THEN OLD.name
                      ELSE OLD.dir || '/' || OLD.name END;
END;

-- Auto-sync triggers for dirs.
CREATE TRIGGER search_dir_ai AFTER INSERT ON dirs
BEGIN
    INSERT INTO search_index (text, path, type) VALUES (NEW.path, NEW.path, 'dir');
END;

CREATE TRIGGER search_dir_ad AFTER DELETE ON dirs
BEGIN
    DELETE FROM search_index WHERE path = OLD.path AND type = 'dir';
END;
