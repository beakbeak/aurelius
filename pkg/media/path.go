package media

import (
	"net/url"
	"path/filepath"

	"github.com/beakbeak/aurelius/pkg/mediadb"
)

const favoritesPath = "favorites.m3u"

// libraryToFsPath converts a URL-style path relative to the root of the media
// library to a path in the local file system.
func (ml *Library) libraryToFsPath(libraryPath string) string {
	return filepath.Join(ml.config.RootPath, libraryPath)
}

// libraryToUrlPath converts a library path to the URI of a resource within a collection (e.g., "tracks").
func (ml *Library) libraryToUrlPath(collection string, libraryPath string) string {
	out := &url.URL{Path: ml.config.Prefix}
	out = out.JoinPath(collection, "at:"+url.PathEscape(libraryPath))
	return out.String()
}

// storageToFsPath converts a path relative to the library's configured storage path to an absolute
// path.
func (ml *Library) storageToFsPath(storagePath string) string {
	return filepath.Join(ml.config.StoragePath, storagePath)
}

// cleanLibraryPath is an alias for mediadb.CleanLibraryPath.
var cleanLibraryPath = mediadb.CleanLibraryPath

// joinLibraryPath is an alias for mediadb.JoinLibraryPath.
var joinLibraryPath = mediadb.JoinLibraryPath
