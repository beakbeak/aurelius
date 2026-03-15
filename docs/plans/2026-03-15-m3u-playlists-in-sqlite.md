# Plan: Mirror .m3u Playlists in SQLite Database

## Context

Playlists (.m3u files) in the media library are currently discovered at request time by reading the filesystem in `handleDirInfo`. This plan moves them into the SQLite database, parsed and resolved to track IDs during the existing FS scan/watch pipeline. Playlists with 0 resolved tracks are excluded from `DirInfo` responses. The existing playlist HTTP API continues to work but backed by DB queries instead of filesystem reads.

## 1. Migration v5: New Tables (`pkg/mediadb/schema.go`)

Add a fifth migration creating two tables:

```sql
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
```

- Follows existing `(dir, name)` pattern from tracks.
- Named `m3u_playlists`/`m3u_playlist_tracks` to leave room for a future `playlists` table for non-file-backed playlists.
- No soft-delete needed (no external references like favorites).
- `track_id` references `tracks_with_deletes` so playlist entries survive soft-deletes but auto-cleanup on hard-delete.
- No FTS integration for now (can be added later).

Add `M3UPlaylist` and `ScannedPlaylist` structs:

```go
type M3UPlaylist struct { ID int64; Dir, Name string; Mtime int64 }
type ScannedPlaylist struct { FSEntry; TrackPaths []string }
```

## 2. DB Query Methods (`pkg/mediadb/db.go`)

- `ForEachM3UPlaylist(fn func(*M3UPlaylist) error)` — iterate all playlists (for diffAgainstDB).
- `GetM3UPlaylist(dir, name string) (*M3UPlaylist, error)` — single lookup (for watcher remove events).
- `GetM3UPlaylistsInDir(dir string) ([]M3UPlaylist, error)` — playlists in a directory with at least one resolved track (JOIN through `tracks` view). For DirInfo.
- `GetM3UPlaylistsInDirIncludingEmpty(dir string) ([]M3UPlaylist, error)` — all playlists in a directory regardless of track count. For removeDirRecursive.
- `GetM3UPlaylistTrackCount(dir, name, prefix string) (int, error)` — count resolved tracks, with optional directory prefix filter. JOINs through `tracks` view.
- `GetM3UPlaylistTrackAt(dir, name string, pos int, prefix string) (string, error)` — track library path at position, with optional prefix filter. ORDER BY position, LIMIT 1 OFFSET pos.

## 3. Scanner Pipeline (`pkg/mediadb/scanner.go`)

### Extend ChangeSet and ScanResult

```go
// ChangeSet additions:
AddedPlaylists   []FSEntry
ChangedPlaylists []FSEntry
RemovedPlaylists []M3UPlaylist

// ScanResult additions:
AddedPlaylists   []ScannedPlaylist
ChangedPlaylists []ScannedPlaylist
RemovedPlaylists []M3UPlaylist
```

### walkFilesystem

Change return type to also return `map[string]FSEntry` for playlists. Switch on `GetFileType(name)`:
- `FileTypeTrack` → existing track map
- `FileTypePlaylist` → new playlist map

### diffAgainstDB

Add playlist diffing after track/dir diffing: iterate `ForEachM3UPlaylist`, compare mtime, categorize into Changed/Removed, remaining are Added.

### M3U Parsing

New function `parseM3U(fsPath string) ([]string, error)`:
- Read file line by line.
- Trim whitespace from each line.
- Skip empty lines and lines starting with `#`.
- Return remaining lines as relative paths.

New function `resolvePlaylistTracks(playlistDir string, lines []string) []string`:
- For each line, `path.Join(playlistDir, line)` and clean.
- Return resolved library paths (actual track_id resolution happens in Apply via INSERT...SELECT).

### collectMetadata

After existing track metadata collection, parse added/changed playlists and build `ScannedPlaylist` entries with resolved track paths.

### Apply

**Order**: Apply tracks first (moves → changed → added → removed), then dirs, then playlists. This ensures newly added tracks exist when playlist track references are resolved.

- **Removed playlists**: `DELETE FROM m3u_playlists WHERE dir = ? AND name = ?` (CASCADE removes playlist_tracks).
- **Changed playlists**: Update mtime, delete all m3u_playlist_tracks for the playlist, re-insert with `INSERT INTO m3u_playlist_tracks (playlist_id, position, track_id) SELECT p.id, ?, t.id FROM m3u_playlists p, tracks t WHERE p.dir = ? AND p.name = ? AND t.dir = ? AND t.name = ?`. The SELECT returns 0 rows for unresolvable tracks, silently skipping them.
- **Added playlists**: Insert playlist row, get lastInsertId, then insert tracks same way as changed.

## 4. Watcher (`pkg/mediadb/watcher.go`)

### processFileEvent

Change the `GetFileType(name) != FileTypeTrack` guard to a switch. Add `FileTypePlaylist` case calling new `processPlaylistEvent` method:

- **Created/Modified**: stat file, build FSEntry, append to AddedPlaylists or ChangedPlaylists.
- **Removed**: look up playlist in DB via `GetM3UPlaylist`, append to RemovedPlaylists.

### processBatch

- Update emptiness check to include playlist changes.
- Update slog.Info to include playlist counts.

### removeDirRecursive

Add removal of playlists in the directory via `GetM3UPlaylistsInDirIncludingEmpty`.

## 5. DirInfo (`pkg/media/dir.go`)

Replace the filesystem-based playlist discovery block (lines 69-87) with:

```go
dbPlaylists, err := ml.db.GetM3UPlaylistsInDir(dirLibraryPath)
// ... error handling ...
for _, p := range dbPlaylists {
    playlistPath := joinLibraryPath(p.Dir, p.Name)
    result.Playlists = append(result.Playlists, PathUrl{
        Name: p.Name,
        Url:  ml.libraryToUrlPath("playlists", playlistPath),
    })
}
```

This automatically excludes playlists with 0 resolved tracks per the `GetM3UPlaylistsInDir` query.

## 6. Playlist API (`pkg/media/playlist.go`, `pkg/media/library.go`)

Replace filesystem-backed playlist handlers with DB-backed versions:

- `handlePlaylistInfoDB(libraryPath, w, req)` — calls `GetM3UPlaylistTrackCount`.
- `handlePlaylistTrackDB(libraryPath, pos, w, req)` — calls `GetM3UPlaylistTrackAt`.

Update `handlePlaylistInfoWrapper` and `handlePlaylistTrackWrapper` in library.go to call the new DB-backed methods.

Remove `playlistCache`, `loadPlaylist`, and the `playlist` struct (dead code after this change).

## 7. Track Changes and Playlist Staleness

When tracks are soft-deleted, `GetM3UPlaylistTrackCount`/`GetM3UPlaylistTrackAt` automatically exclude them (they JOIN through the `tracks` view). Track moves update rows in-place (same ID), so playlist references remain valid. New tracks not yet referenced will be picked up when the .m3u file changes or on next full rescan. No special re-resolution needed.

## Files to Modify

1. `pkg/mediadb/schema.go` — migration v5, M3UPlaylist/ScannedPlaylist types
2. `pkg/mediadb/db.go` — playlist query methods
3. `pkg/mediadb/scanner.go` — ChangeSet/ScanResult extensions, walkFilesystem, diffAgainstDB, parseM3U, resolvePlaylistTracks, collectMetadata, Apply
4. `pkg/mediadb/watcher.go` — processPlaylistEvent, processFileEvent switch, processBatch check, removeDirRecursive
5. `pkg/media/dir.go` — replace filesystem playlist discovery with DB query
6. `pkg/media/playlist.go` — DB-backed handlers, remove filesystem-backed code
7. `pkg/media/library.go` — update wrappers, remove playlistCache

## Verification

1. `go build -tags sqlite_fts5` from `cmd/aurelius` — must compile.
2. `go test -tags sqlite_fts5 -asan ./...` — existing tests must pass.
3. `golangci-lint run` — no lint errors.
4. Manual: place .m3u files in media library, verify they appear in DirInfo, verify playlist API returns correct tracks, verify playlists with no valid tracks are excluded.
5. Manual: add/modify/remove .m3u files while running, verify watcher picks up changes.
