-- v4: FTS5 path search index.
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
WHEN OLD.dir != NEW.dir OR OLD.name != NEW.name OR OLD.deleted != NEW.deleted
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
END;
