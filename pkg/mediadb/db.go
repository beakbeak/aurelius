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
	var tagsJSON, imagesJSON, metadataJSON string

	err := row.Scan(
		&t.ID, &t.Dir, &t.Name, &t.Mtime, &t.Hash,
		&tagsJSON, &imagesJSON, &metadataJSON,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(tagsJSON), &t.Tags); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
	}
	if err := json.Unmarshal([]byte(imagesJSON), &t.AttachedImages); err != nil {
		return nil, fmt.Errorf("failed to unmarshal attached_images: %w", err)
	}
	if err := json.Unmarshal([]byte(metadataJSON), &t.Metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return &t, nil
}

const trackColumns = `id, dir, name, mtime, hash, tags, attached_images, metadata`

// GetTrack returns the track at the given library path (dir + name).
func (db *DB) GetTrack(dir, name string) (*Track, error) {
	row := db.db.QueryRow(
		`SELECT `+trackColumns+` FROM tracks WHERE dir = ? AND name = ?`,
		dir, name,
	)
	t, err := scanTrack(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return t, err
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

// GetM3UPlaylist returns the m3u playlist at the given library path (dir + name).
func (db *DB) GetM3UPlaylist(dir, name string) (*M3UPlaylist, error) {
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

// m3uPlaylistDirFilter returns a SQL WHERE clause fragment and args that filter
// playlist tracks by directory prefix. If prefix is empty, no filter is applied.
func m3uPlaylistDirFilter(prefix string) (string, []any) {
	if prefix == "" {
		return "", nil
	}
	prefix = CleanLibraryPath(prefix)
	return `AND (t.dir = ? OR t.dir LIKE ? || '/%')`, []any{prefix, prefix}
}

// GetM3UPlaylistTrackCount returns the number of resolved tracks in a playlist.
// Only counts tracks visible through the tracks view (not soft-deleted).
func (db *DB) GetM3UPlaylistTrackCount(dir, name, prefix string) (int, error) {
	filter, filterArgs := m3uPlaylistDirFilter(prefix)
	args := []any{dir, name}
	args = append(args, filterArgs...)
	var count int
	err := db.db.QueryRow(
		`SELECT COUNT(*) FROM m3u_playlist_tracks pt
		JOIN tracks t ON t.id = pt.track_id
		JOIN m3u_playlists p ON p.id = pt.playlist_id
		WHERE p.dir = ? AND p.name = ? `+filter,
		args...,
	).Scan(&count)
	return count, err
}

// GetM3UPlaylistTrackAt returns the library path of the track at the given
// position in a playlist. Only considers tracks visible through the tracks view.
// Returns ("", nil) if pos is out of range.
func (db *DB) GetM3UPlaylistTrackAt(dir, name string, pos int, prefix string) (string, error) {
	if pos < 0 {
		return "", nil
	}
	filter, filterArgs := m3uPlaylistDirFilter(prefix)
	args := []any{dir, name}
	args = append(args, filterArgs...)
	args = append(args, pos)
	var libraryPath string
	err := db.db.QueryRow(
		`SELECT CASE WHEN t.dir = '' THEN t.name
			ELSE t.dir || '/' || t.name END
		FROM m3u_playlist_tracks pt
		JOIN tracks t ON t.id = pt.track_id
		JOIN m3u_playlists p ON p.id = pt.playlist_id
		WHERE p.dir = ? AND p.name = ? `+filter+`
		ORDER BY pt.position
		LIMIT 1 OFFSET ?`,
		args...,
	).Scan(&libraryPath)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return libraryPath, err
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
