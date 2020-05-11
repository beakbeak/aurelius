package media

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func (db *Library) toFileSystemPath(dbPath string) string {
	return filepath.Join(db.root, dbPath)
}

func (db *Library) toLibraryPath(fsPath string) (string, error) {
	dbPath, err := filepath.Rel(db.root, fsPath)
	if err != nil {
		return "", err
	}

	dbPath = filepath.ToSlash(dbPath)

	if strings.HasPrefix(dbPath, "..") || strings.HasPrefix(dbPath, "/") {
		return "", fmt.Errorf("path is not under media library root")
	}
	return dbPath, nil
}

func (db *Library) toLibraryPathWithContext(fsPath, context string) (string, error) {
	realContext, err := filepath.EvalSymlinks(context)
	if err != nil {
		return "", err
	}

	fsPathInContext, err := filepath.Rel(realContext, fsPath)
	if err != nil {
		return "", err
	}
	fsPathInContext = filepath.Join(context, fsPathInContext)

	dbPathInContext, err := db.toLibraryPath(fsPathInContext)
	if err == nil {
		// contextualized path may be invalid if resolved path is at a
		// different level of indirection than context
		if _, err := os.Stat(db.toFileSystemPath(dbPathInContext)); err == nil {
			return dbPathInContext, nil
		}
	}
	return db.toLibraryPath(fsPath)
}

func (db *Library) toUrlPath(dbPath string) string {
	urlPath := path.Join(db.prefix, dbPath)
	return (&url.URL{Path: urlPath}).String()
}

func (db *Library) toHtmlPath(path string) string {
	return filepath.Join(db.htmlPath, path)
}
