package media

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const treePrefix = "tree"
const favoritesPath = "favorites.m3u"

// libraryToFsPath converts a URL-style path relative to the root of the media
// library to a path in the local file system.
func (ml *Library) libraryToFsPath(libraryPath string) string {
	return filepath.Join(ml.config.RootPath, libraryPath)
}

// fsToLibraryPath converts a path in the local file system to a URL-style path
// relative to the root of the media library.
func (ml *Library) fsToLibraryPath(fsPath string) (string, error) {
	libraryPath, err := filepath.Rel(ml.config.RootPath, fsPath)
	if err != nil {
		return "", err
	}

	libraryPath = filepath.ToSlash(libraryPath)

	if strings.HasPrefix(libraryPath, "..") || strings.HasPrefix(libraryPath, "/") {
		return "", fmt.Errorf("path is not under media library root")
	}
	return libraryPath, nil
}

// fsToLibraryPathWithContext converts a path in the local file system to a
// URL-style path relative to the root of the media library.
//
// The context parameter is an ancestor directory of fsPath. The function uses
// context as a basis for resolving symbolic links: it attempts to resolve
// symbolic links below context and preserve symbolic links above context.
func (ml *Library) fsToLibraryPathWithContext(fsPath, context string) (string, error) {
	realFsPath, err := filepath.EvalSymlinks(fsPath)
	if err != nil {
		return "", err
	}

	realContext, err := filepath.EvalSymlinks(context)
	if err != nil {
		return "", err
	}

	fsPathInContext, err := filepath.Rel(realContext, realFsPath)
	if err != nil {
		return "", err
	}
	fsPathInContext = filepath.Join(context, fsPathInContext)

	libraryPathInContext, err := ml.fsToLibraryPath(fsPathInContext)
	if err == nil {
		// contextualized path may be invalid if resolved path is at a
		// different level of indirection than context
		if _, err := os.Stat(ml.libraryToFsPath(libraryPathInContext)); err == nil {
			return libraryPathInContext, nil
		}
	}
	return ml.fsToLibraryPath(fsPath)
}

// libraryToUrlPath prepends the library's routing prefix to libraryPath and applies
// URL encoding to the result.
func (ml *Library) libraryToUrlPath(libraryPath string) string {
	urlPath := path.Join(ml.config.Prefix, treePrefix, libraryPath)
	return (&url.URL{Path: urlPath}).String()
}

// storageToFsPath converts a path relative to the library's configured storage path to an absolute
// path.
func (ml *Library) storageToFsPath(storagePath string) string {
	return filepath.Join(ml.config.StoragePath, storagePath)
}
