package media

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func (db *Database) toFileSystemPath(dbPath string) string {
	return filepath.Join(db.root, dbPath)
}

func (db *Database) toDatabasePath(fsPath string) (string, error) {
	dbPath, err := filepath.Rel(db.root, fsPath)
	if err != nil {
		return "", err
	}

	dbPath = filepath.ToSlash(dbPath)

	if strings.HasPrefix(dbPath, "..") || strings.HasPrefix(dbPath, "/") {
		return "", fmt.Errorf("path is not under database root")
	}
	return dbPath, nil
}

func (db *Database) toDatabasePathWithContext(fsPath, context string) (string, error) {
	realContext, err := filepath.EvalSymlinks(context)
	if err != nil {
		return "", err
	}

	fsPathInContext, err := filepath.Rel(realContext, fsPath)
	if err != nil {
		return "", err
	}
	fsPathInContext = filepath.Join(context, fsPathInContext)

	dbPathInContext, err := db.toDatabasePath(fsPathInContext)
	if err == nil {
		// contextualized path may be invalid if resolved path is at a
		// different level of indirection than context
		if _, err := os.Stat(db.toFileSystemPath(dbPathInContext)); err == nil {
			return dbPathInContext, nil
		}
	}
	return db.toDatabasePath(fsPath)
}

func (db *Database) toUrlPath(dbPath string) string {
	urlPath := path.Join(db.prefix, dbPath)
	return (&url.URL{Path: urlPath}).String()
}

func (db *Database) toHtmlPath(path string) string {
	return filepath.Join(db.htmlPath, path)
}
