package mediadb

import (
	"path"
	"strings"
)

// CleanLibraryPath normalizes a library path by calling path.Clean and
// replacing "." with "".
func CleanLibraryPath(libraryPath string) string {
	cleanedPath := path.Clean(strings.Trim(libraryPath, "/"))
	if cleanedPath == "." {
		return ""
	}
	return cleanedPath
}

// JoinLibraryPath joins a directory and filename into a library path.
func JoinLibraryPath(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + "/" + name
}
