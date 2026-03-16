package mediadb

import (
	"regexp"
	"strings"

	"github.com/beakbeak/aurelius/pkg/aurelib"
)

// FileType classifies files in the media library.
type FileType int

const (
	FileTypeIgnored  FileType = iota
	FileTypePlaylist
	FileTypeTrack
	FileTypeImage
)

var (
	reFragment = regexp.MustCompile(`(?i)\.[0-9]+\.txt$`)
	rePlaylist = regexp.MustCompile(`(?i)\.m3u$`)
	reImage    = regexp.MustCompile(`(?i)\.(jpg|jpeg|png|gif)$`)
	reIgnore   = regexp.MustCompile(`(?i)\.(txt|nfo)$`)
	reTrack    = regexp.MustCompile(`(?i)\.(opus|m4a|wma|wmv|wav|` + strings.Join(aurelib.InputExtensions(), "|") + `)$`)
)

// GetFileType determines a file's type.
func GetFileType(filename string) FileType {
	switch {
	case reFragment.MatchString(filename):
		return FileTypeTrack
	case rePlaylist.MatchString(filename):
		return FileTypePlaylist
	case reImage.MatchString(filename):
		return FileTypeImage
	case reIgnore.MatchString(filename):
		return FileTypeIgnored
	case reTrack.MatchString(filename):
		return FileTypeTrack
	default:
		return FileTypeIgnored
	}
}
