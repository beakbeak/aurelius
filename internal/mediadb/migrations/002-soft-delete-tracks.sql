-- v2: Soft-delete tracks.
ALTER TABLE tracks RENAME TO tracks_with_deletes;
ALTER TABLE tracks_with_deletes ADD COLUMN deleted INTEGER NOT NULL DEFAULT 0;
CREATE VIEW tracks AS
  SELECT id, dir, name, mtime, hash, tags, attached_images, metadata
  FROM tracks_with_deletes
  WHERE deleted = 0;
