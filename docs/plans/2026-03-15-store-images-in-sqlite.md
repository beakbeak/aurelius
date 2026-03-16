# Plan: Store Images in SQLite Database

## Context

Currently, track images (both attached to audio files and loose image files in directories) are loaded from the filesystem on every HTTP request. Attached image metadata (mime type + size) is stored in a JSON column on the tracks table, but binary data is not persisted. Directory images are discovered at request time. The frontend also filters out large images client-side. This plan moves all image data into SQLite, content-addressed by hash, with automatic resize/re-encode for oversized images.

## Schema (v6 migration) — `pkg/mediadb/schema.go`

New tables:

```sql
CREATE TABLE images (
    hash BLOB PRIMARY KEY,
    mime_type TEXT NOT NULL,
    data BLOB NOT NULL
);

CREATE TABLE track_images (
    track_id  INTEGER NOT NULL REFERENCES tracks_with_deletes(id) ON DELETE CASCADE,
    position  INTEGER NOT NULL,
    image_hash BLOB NOT NULL REFERENCES images(hash),
    PRIMARY KEY (track_id, position)
);

-- Trigger: auto-delete orphaned images when track_images rows are removed.
CREATE TRIGGER clean_orphaned_images
AFTER DELETE ON track_images
BEGIN
    DELETE FROM images
    WHERE hash = OLD.image_hash
    AND NOT EXISTS (
        SELECT 1 FROM track_images WHERE image_hash = OLD.image_hash
    );
END;

-- Drop the attached_images column (recreate the view first since it references it).
DROP VIEW tracks;
ALTER TABLE tracks_with_deletes DROP COLUMN attached_images;
CREATE VIEW tracks AS
  SELECT id, dir, name, mtime, hash, tags, metadata
  FROM tracks_with_deletes
  WHERE deleted = 0;

-- Force full rescan to populate images.
UPDATE tracks_with_deletes SET mtime = 0;
```

## Image Processing — new `pkg/mediadb/image.go`

- `processImage(data []byte, mimeType string) ([]byte, string, [32]byte, error)`
  - If data size <= 100KiB: keep original bytes and mime type, no decoding needed
  - If data size > 100KiB: decode with `image.Decode()`, resize to fit 1024x1024 (aspect-preserving, only if larger) using `golang.org/x/image/draw.CatmullRom` with `draw.Src` op, then encode as JPEG with iteratively decreasing quality until <= 100KiB (start at 85, step down by 10, minimum 15)
  - Return: processed data, mime type, SHA-256 hash

- `processImageFile(path string, mimeType string) ([]byte, string, [32]byte, error)`
  - Read file from disk, call `processImage()`, return result
  - Processes one file at a time to avoid loading many large images into memory

- `collectDirectoryImagePaths(trackFsPath string) []directoryImageRef`
  - Move sorting/discovery logic from `pkg/media/track.go`'s `getDirectoryImages()` here
  - Same cover-name-first sorting, NO size skip threshold (images will be resized)
  - Returns file paths and mime types only, does NOT read file contents
  - `directoryImageRef{path string, mimeType string}`

## Scanner Changes — `pkg/mediadb/scanner.go`

**Types:**
- Remove `AttachedImages` from `ScannedTrack` entirely — no image data or references stored between phases
- `ScannedImage{Hash [32]byte, MimeType string, Data []byte}` — transient, used only within `Apply()`

**`scanFile()`:**
- No longer collects any image data or references. Only extracts tags, metadata, and hash as before.

**`marshalTrackJSON()`:**
- Remove `attached_images` marshaling entirely (column dropped in v6)

**New helper: `collectAndInsertTrackImages()`** (called from `Apply()`):
- Takes: `tx`, track ID, filesystem path
- Opens audio source, extracts attached images one at a time, processes each through `processImage()`, inserts into `images` + `track_images`, then moves on to next (no accumulation). If the audio source cannot be opened, returns early (skips directory images for unusable tracks).
- Calls `collectDirectoryImagePaths()` for the directory, then processes each file one at a time via `processImageFile()`, inserts, moves on
- `logImageProcessingError()` helper lives in this file alongside the caller
- Deduplicates by hash: skip `track_images` insertion if a previous position used the same hash
- Each image is loaded → processed → inserted → released before the next one

**`Apply()`:**
- Remove `attached_images` from INSERT/UPDATE SQL statements
- After inserting/updating a track:
  - For changed tracks: `DELETE FROM track_images WHERE track_id = ?` first
  - Call `collectAndInsertTrackImages(tx, trackID, fsPath)` to process images one at a time
  - Need track IDs: use `RETURNING id` on INSERT, or `SELECT id` for updates
- Orphaned images are cleaned up automatically by the `clean_orphaned_images` trigger when `track_images` rows are deleted

## Watcher Changes — `pkg/mediadb/watcher.go` + `pkg/mediadb/filetype.go`

- Add `FileTypeImage` to `filetype.go` matching `.jpg`, `.jpeg`, `.png`, `.gif`
- Remove `.gif` from `reIgnore` pattern (currently `.gif` is in the ignore list)
- Add `reImage` pattern before `reIgnore` in `GetFileType()` switch
- In `processFileEvent()`, handle `FileTypeImage` events
- `processImageFileEvent()`: look up all tracks in the same directory via `db.GetTracksInDir(dir)`, mark each as `Changed` in the `ChangeSet` (this reuses the existing full re-scan path — simpler code, and image file changes are rare)

## DB Query Changes — `pkg/mediadb/db.go`

- `GetTrackImageData(dir, name string, position int) (data []byte, mimeType string, hash []byte, err error)` — takes library path directly, loads full blob data, only called when serving an image
- Update `scanTrack()`: remove `attached_images` from column list and scan; update `trackColumns` const
- Update `GetTrack()` to populate `Images` from `track_images` join — loads only `hash` and `mime_type`, NOT the blob `data`
- New `ImageInfo{Hash []byte, MimeType string}` type replaces `AttachedImageInfo` on `Track` struct
- `GetTracksInDir()` / `ForEachTrack()` do NOT load images (not needed for those callers)

## Serving Changes — `pkg/media/track.go`

**`handleTrackImage()`:**
- Query `GetTrackImageData(dir, name, index)` directly using the library path
- Use hex-encoded content hash as ETag (stronger than current size-based ETag)
- Serve blob with `Content-Type` and `Cache-Control` headers

**`handleTrackInfo()`:**
- Remove directory image discovery (`getDirectoryImages` call)
- Build image list from `track.Images` (populated from DB)
- Response JSON still has `mimeType` and `size` fields; `size` comes from `length(data)` in the `track_images`/`images` join query (computed by SQLite, not loaded into Go memory)

**Remove:** `getDirectoryImages()`, `getAttachedAndDirectoryImages()`, `attachedOrDirectoryImage`, `directoryImage` types, `coverImageRegex`, `maxDirectoryImageSize`

## Frontend Changes — `ts/ui/player.ts`

- Remove `filterTrackImages()` function
- Remove `maxImageSize` constant
- In `updateStatus()`: use `info.attachedImages` directly instead of `filterTrackImages(info.attachedImages)`
- Image URLs become `${track.url}/images/${index}` (no `originalIndex` indirection)
- Remove `StreamCodec` import if it becomes unused

## Dependencies

- Add `golang.org/x/image` to `go.mod` (for `draw` package)

## Files to Modify

1. `pkg/mediadb/schema.go` — v6 migration, new types, update `AttachedImageInfo` → `ImageInfo`
2. `pkg/mediadb/image.go` — **new file**, image processing + directory image collection
3. `pkg/mediadb/scanner.go` — `ScannedTrack`, `scanFile()`, `marshalTrackJSON()`, `Apply()`
4. `pkg/mediadb/db.go` — new queries, update `GetTrack()`, `scanTrack()`
5. `pkg/mediadb/filetype.go` — add `FileTypeImage`
6. `pkg/mediadb/watcher.go` — handle image file events
7. `pkg/media/track.go` — rewrite image serving, remove filesystem image code
8. `ts/ui/player.ts` — remove `filterTrackImages`, `maxImageSize`, simplify image URLs
9. `go.mod` / `go.sum` — add `golang.org/x/image`

## Verification

1. `go build -tags sqlite_fts5` from `cmd/aurelius`
2. `golangci-lint run`
3. `go test -tags sqlite_fts5 -asan ./...`
4. `npm run test`
5. Manual: start server, verify track images load in browser, verify ETag caching works
