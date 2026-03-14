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
