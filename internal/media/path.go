package media

import (
	"encoding/hex"
	"net/url"
	"path/filepath"

	"github.com/beakbeak/aurelius/internal/mediadb"
)

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

// makeImageUrl builds an absolute URL path for an image identified by its content hash.
func (ml *Library) makeImageUrl(imageHash []byte) string {
	out := &url.URL{Path: ml.config.Prefix}
	out = out.JoinPath("images", "hash:"+hex.EncodeToString(imageHash))
	return out.String()
}

// cleanLibraryPath is an alias for mediadb.CleanLibraryPath.
var cleanLibraryPath = mediadb.CleanLibraryPath

// joinLibraryPath is an alias for mediadb.JoinLibraryPath.
var joinLibraryPath = mediadb.JoinLibraryPath
