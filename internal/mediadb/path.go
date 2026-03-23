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

// SplitLibraryPath splits a library path into its directory and name components.
func SplitLibraryPath(libraryPath string) (dir, name string) {
	dir, name = path.Split(libraryPath)
	dir = CleanLibraryPath(dir)
	return dir, name
}

// JoinLibraryPath joins a directory and filename into a library path.
func JoinLibraryPath(dir, name string) string {
	if dir == "" {
		return name
	}
	return dir + "/" + name
}
