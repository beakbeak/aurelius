-- v6: Store images in database. Orphan cleanup is done at DB open, not via
-- trigger. Also optimizes the path search update trigger to skip no-op
-- updates (mtime, hash, tags, metadata changes).
CREATE TABLE images (
    hash          BLOB PRIMARY KEY,
    original_hash BLOB NOT NULL,
    mime_type     TEXT NOT NULL,
    data          BLOB NOT NULL
);

CREATE INDEX idx_images_original_hash ON images(original_hash);

CREATE TABLE track_images (
    track_id   INTEGER NOT NULL REFERENCES tracks_with_deletes(id) ON DELETE CASCADE,
    position   INTEGER NOT NULL,
    image_hash BLOB NOT NULL REFERENCES images(hash),

    PRIMARY KEY (track_id, position)
);

-- Drop the attached_images column (recreate the view first since it references it).
DROP VIEW tracks;
ALTER TABLE tracks_with_deletes DROP COLUMN attached_images;
CREATE VIEW tracks AS
  SELECT id, dir, name, mtime, hash, tags, metadata
  FROM tracks_with_deletes
  WHERE deleted = 0;

-- Recreate path search trigger with WHEN clause to skip no-op updates.
DROP TRIGGER path_search_track_au;
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

-- Force full rescan to populate images.
UPDATE tracks_with_deletes SET mtime = 0;
