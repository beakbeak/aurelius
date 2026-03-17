// Package mediadb provides a SQLite database for persisting metadata about
// files in a media library.
package mediadb

import (
	"database/sql"
	"encoding/json"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps a SQLite database for the media library.
type DB struct {
	db *sql.DB
}

// Open opens or creates the database at the given path.
func Open(path string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := migrate(sqlDB); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	// Clean up images no longer referenced by any track.
	if _, err := sqlDB.Exec("DELETE FROM images WHERE NOT EXISTS (SELECT 1 FROM track_images WHERE image_hash = images.hash)"); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to clean orphaned images: %w", err)
	}

	return &DB{db: sqlDB}, nil
}

// migrate applies any pending migrations based on PRAGMA user_version.
func migrate(db *sql.DB) error {
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("failed to read user_version: %w", err)
	}

	for i := version; i < len(migrations); i++ {
		if _, err := db.Exec(migrations[i]); err != nil {
			return fmt.Errorf("migration %d failed: %w", i+1, err)
		}
	}

	if _, err := db.Exec(fmt.Sprintf("PRAGMA user_version = %d", len(migrations))); err != nil {
		return fmt.Errorf("failed to update user_version: %w", err)
	}
	return nil
}

// Close closes the database.
func (db *DB) Close() error {
	return db.db.Close()
}

func scanTrack(row interface{ Scan(...any) error }) (*Track, error) {
	var t Track
	var tagsJSON, metadataJSON string

	err := row.Scan(
		&t.ID, &t.Dir, &t.Name, &t.Mtime, &t.Hash,
		&tagsJSON, &metadataJSON,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(tagsJSON), &t.Tags); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
	}
	if err := json.Unmarshal([]byte(metadataJSON), &t.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &t, nil
}

const trackColumns = `id, dir, name, mtime, hash, tags, metadata`

// GetTrack returns the track at the given library path,
// including image metadata (but not image data).
func (db *DB) GetTrack(libraryPath string) (*Track, error) {
	dir, name := SplitLibraryPath(libraryPath)
	row := db.db.QueryRow(
		`SELECT `+trackColumns+` FROM tracks WHERE dir = ? AND name = ?`,
		dir, name,
	)
	t, err := scanTrack(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	images, err := db.getTrackImages(t.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load track images: %w", err)
	}
	t.Images = images
	return t, nil
}

// getTrackImages returns image metadata for a track (hash, mime type, size)
// without loading the image data blob.
func (db *DB) getTrackImages(trackID int64) ([]ImageInfo, error) {
	rows, err := db.db.Query(
		`SELECT i.hash, i.mime_type, length(i.data)
		FROM track_images ti
		JOIN images i ON i.hash = ti.image_hash
		WHERE ti.track_id = ?
		ORDER BY ti.position`,
		trackID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var images []ImageInfo
	for rows.Next() {
		var img ImageInfo
		if err := rows.Scan(&img.Hash, &img.MimeType, &img.Size); err != nil {
			return nil, err
		}
		images = append(images, img)
	}
	return images, rows.Err()
}

// GetTrackImageData returns the full image data for a track at the given
// position. Returns (nil, "", nil, nil) if not found.
func (db *DB) GetTrackImageData(libraryPath string, position int) (data []byte, mimeType string, hash []byte, err error) {
	dir, name := SplitLibraryPath(libraryPath)
	err = db.db.QueryRow(
		`SELECT i.data, i.mime_type, i.hash
		FROM track_images ti
		JOIN images i ON i.hash = ti.image_hash
		JOIN tracks t ON t.id = ti.track_id
		WHERE t.dir = ? AND t.name = ? AND ti.position = ?`,
		dir, name, position,
	).Scan(&data, &mimeType, &hash)
	if err == sql.ErrNoRows {
		return nil, "", nil, nil
	}
	return
}

// GetTracksInDir returns all tracks in the given directory.
func (db *DB) GetTracksInDir(dir string) ([]Track, error) {
	rows, err := db.db.Query(
		`SELECT `+trackColumns+` FROM tracks WHERE dir = ? ORDER BY name`,
		dir,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []Track
	for rows.Next() {
		t, err := scanTrack(rows)
		if err != nil {
			return nil, err
		}
		tracks = append(tracks, *t)
	}
	return tracks, rows.Err()
}

// GetSubdirs returns all immediate subdirectories of the given directory.
func (db *DB) GetSubdirs(parent string) ([]Dir, error) {
	rows, err := db.db.Query(
		`SELECT path, parent FROM dirs WHERE parent = ? ORDER BY path`,
		parent,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dirs []Dir
	for rows.Next() {
		var d Dir
		if err := rows.Scan(&d.Path, &d.Parent); err != nil {
			return nil, err
		}
		dirs = append(dirs, d)
	}
	return dirs, rows.Err()
}

// ForEachTrack iterates over all tracks, calling fn for each.
func (db *DB) ForEachTrack(fn func(*Track) error) error {
	rows, err := db.db.Query(
		`SELECT ` + trackColumns + ` FROM tracks`,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		t, err := scanTrack(rows)
		if err != nil {
			return err
		}
		if err := fn(t); err != nil {
			return err
		}
	}
	return rows.Err()
}

// AllDirs returns all directories in the database.
func (db *DB) AllDirs() (map[string]Dir, error) {
	rows, err := db.db.Query(`SELECT path, parent FROM dirs`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dirs := make(map[string]Dir)
	for rows.Next() {
		var d Dir
		if err := rows.Scan(&d.Path, &d.Parent); err != nil {
			return nil, err
		}
		dirs[d.Path] = d
	}
	return dirs, rows.Err()
}

// IsFavorite reports whether the track at the given library path is a favorite.
func (db *DB) IsFavorite(libraryPath string) (bool, error) {
	dir, name := SplitLibraryPath(libraryPath)
	var exists bool
	err := db.db.QueryRow(
		`SELECT EXISTS(
			SELECT 1 FROM favorites
			JOIN tracks ON tracks.id = favorites.track_id
			WHERE tracks.dir = ? AND tracks.name = ?
		)`,
		dir, name,
	).Scan(&exists)
	return exists, err
}

// SetFavorite adds or removes a favorite for the track at the given library path.
func (db *DB) SetFavorite(libraryPath string, favorite bool) error {
	dir, name := SplitLibraryPath(libraryPath)
	if favorite {
		_, err := db.db.Exec(
			`INSERT OR IGNORE INTO favorites(track_id)
			SELECT id FROM tracks WHERE dir = ? AND name = ?`,
			dir, name,
		)
		return err
	}
	_, err := db.db.Exec(
		`DELETE FROM favorites WHERE track_id = (
			SELECT id FROM tracks WHERE dir = ? AND name = ?
		)`,
		dir, name,
	)
	return err
}

// ForEachM3UPlaylist iterates over all m3u playlists, calling fn for each.
func (db *DB) ForEachM3UPlaylist(fn func(*M3UPlaylist) error) error {
	rows, err := db.db.Query(
		`SELECT id, dir, name, mtime FROM m3u_playlists`,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var p M3UPlaylist
		if err := rows.Scan(&p.ID, &p.Dir, &p.Name, &p.Mtime); err != nil {
			return err
		}
		if err := fn(&p); err != nil {
			return err
		}
	}
	return rows.Err()
}

// GetM3UPlaylist returns the m3u playlist at the given library path.
func (db *DB) GetM3UPlaylist(libraryPath string) (*M3UPlaylist, error) {
	dir, name := SplitLibraryPath(libraryPath)
	var p M3UPlaylist
	err := db.db.QueryRow(
		`SELECT id, dir, name, mtime FROM m3u_playlists WHERE dir = ? AND name = ?`,
		dir, name,
	).Scan(&p.ID, &p.Dir, &p.Name, &p.Mtime)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// GetM3UPlaylistsInDir returns playlists in the given directory that have at
// least one resolved track (through the tracks view, excluding soft-deleted).
func (db *DB) GetM3UPlaylistsInDir(dir string) ([]M3UPlaylist, error) {
	rows, err := db.db.Query(
		`SELECT p.id, p.dir, p.name, p.mtime
		FROM m3u_playlists p
		WHERE p.dir = ? AND EXISTS (
			SELECT 1 FROM m3u_playlist_tracks pt
			JOIN tracks t ON t.id = pt.track_id
			WHERE pt.playlist_id = p.id
		)
		ORDER BY p.name`,
		dir,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var playlists []M3UPlaylist
	for rows.Next() {
		var p M3UPlaylist
		if err := rows.Scan(&p.ID, &p.Dir, &p.Name, &p.Mtime); err != nil {
			return nil, err
		}
		playlists = append(playlists, p)
	}
	return playlists, rows.Err()
}

// GetM3UPlaylistsInDirIncludingEmpty returns all playlists in the given
// directory regardless of whether they have resolved tracks.
func (db *DB) GetM3UPlaylistsInDirIncludingEmpty(dir string) ([]M3UPlaylist, error) {
	rows, err := db.db.Query(
		`SELECT id, dir, name, mtime FROM m3u_playlists WHERE dir = ? ORDER BY name`,
		dir,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var playlists []M3UPlaylist
	for rows.Next() {
		var p M3UPlaylist
		if err := rows.Scan(&p.ID, &p.Dir, &p.Name, &p.Mtime); err != nil {
			return nil, err
		}
		playlists = append(playlists, p)
	}
	return playlists, rows.Err()
}

// GetM3UPlaylistTrackCount returns the number of resolved tracks in a playlist.
// Only counts tracks visible through the tracks view (not soft-deleted).
func (db *DB) GetM3UPlaylistTrackCount(libraryPath string) (int, error) {
	dir, name := SplitLibraryPath(libraryPath)
	var count int
	err := db.db.QueryRow(
		`SELECT COUNT(*) FROM m3u_playlist_tracks pt
		JOIN tracks t ON t.id = pt.track_id
		JOIN m3u_playlists p ON p.id = pt.playlist_id
		WHERE p.dir = ? AND p.name = ?`,
		dir, name,
	).Scan(&count)
	return count, err
}

// GetM3UPlaylistTrackAt returns the library path of the track at the given
// position in a playlist. Only considers tracks visible through the tracks view.
// Returns ("", nil) if pos is out of range.
func (db *DB) GetM3UPlaylistTrackAt(playlistPath string, pos int) (string, error) {
	dir, name := SplitLibraryPath(playlistPath)
	if pos < 0 {
		return "", nil
	}
	var libraryPath string
	err := db.db.QueryRow(
		`SELECT CASE WHEN t.dir = '' THEN t.name
			ELSE t.dir || '/' || t.name END
		FROM m3u_playlist_tracks pt
		JOIN tracks t ON t.id = pt.track_id
		JOIN m3u_playlists p ON p.id = pt.playlist_id
		WHERE p.dir = ? AND p.name = ?
		ORDER BY pt.position
		LIMIT 1 OFFSET ?`,
		dir, name, pos,
	).Scan(&libraryPath)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return libraryPath, err
}

// RecordPlay records a play event for the track at the given library path.
func (db *DB) RecordPlay(libraryPath string) error {
	dir, name := SplitLibraryPath(libraryPath)
	_, err := db.db.Exec(
		`INSERT INTO play_history(track_id)
		SELECT id FROM tracks WHERE dir = ? AND name = ?`,
		dir, name,
	)
	return err
}

// PlayCount returns the number of play history entries for the given track.
func (db *DB) PlayCount(libraryPath string) (int, error) {
	dir, name := SplitLibraryPath(libraryPath)
	var count int
	err := db.db.QueryRow(
		`SELECT COUNT(*) FROM play_history
		JOIN tracks ON play_history.track_id = tracks.id
		WHERE tracks.dir = ? AND tracks.name = ?`,
		dir, name,
	).Scan(&count)
	return count, err
}

// favoriteDirFilter returns a SQL WHERE clause and args that filter favorites
// by directory prefix. If prefix is empty, no filter is applied.
func favoriteDirFilter(prefix string) (string, []any) {
	if prefix == "" {
		return "", nil
	}
	prefix = CleanLibraryPath(prefix)
	return `WHERE tracks.dir = ? OR tracks.dir LIKE ? || '/%'`, []any{prefix, prefix}
}

// CountFavorites returns the number of favorite tracks. If prefix is non-empty,
// only favorites whose directory matches the prefix are counted.
func (db *DB) CountFavorites(prefix string) (int, error) {
	where, args := favoriteDirFilter(prefix)
	var count int
	err := db.db.QueryRow(
		`SELECT COUNT(*) FROM favorites
		JOIN tracks ON tracks.id = favorites.track_id `+where,
		args...,
	).Scan(&count)
	return count, err
}

// GetFavoriteAt returns the library path of the favorite at the given position
// (ordered by rowid). If prefix is non-empty, only favorites whose directory
// matches the prefix are considered. Returns ("", nil) if pos is out of range.
func (db *DB) GetFavoriteAt(pos int, prefix string) (string, error) {
	where, args := favoriteDirFilter(prefix)
	args = append(args, pos)
	var libraryPath string
	err := db.db.QueryRow(
		`SELECT CASE WHEN tracks.dir = '' THEN tracks.name
			ELSE tracks.dir || '/' || tracks.name END
		FROM favorites
		JOIN tracks ON tracks.id = favorites.track_id
		`+where+`
		ORDER BY favorites.rowid
		LIMIT 1 OFFSET ?`,
		args...,
	).Scan(&libraryPath)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return libraryPath, err
}
