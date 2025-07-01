package media

import (
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
)

var (
	reDirIgnore   = regexp.MustCompile(`(?i)\.(:?jpe?g|png|txt|log|cue|gif|pdf|sfv|nfo|bak)$`)
	reDirUnignore = regexp.MustCompile(`(?i)\.[0-9]+\.txt$`)
	rePlaylist    = regexp.MustCompile(`(?i)\.m3u$`)
)

func (ml *Library) handleDirRequest(
	libraryPath string,
	w http.ResponseWriter,
	req *http.Request,
) bool {
	query := req.URL.Query()

	if _, ok := query["info"]; ok {
		ml.handleDirInfoRequest(libraryPath, w)
		return true
	}

	return false
}

func (ml *Library) handleDirInfoRequest(
	libraryDirPath string,
	w http.ResponseWriter,
) {
	fsDirPath := ml.libraryToFsPath(libraryDirPath)

	entries, err := os.ReadDir(fsDirPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		slog.Error("ReadDir failed", "error", err)
		return
	}

	type PathUrl struct {
		Name string `json:"name"`
		Url  string `json:"url"`
	}

	makeRelativePathUrl := func(name string) PathUrl {
		return PathUrl{
			Name: name,
			Url:  ml.libraryToUrlPath(path.Join(libraryDirPath, name)),
		}
	}

	makeAbsolutePathUrl := func(name, fsPath string) (PathUrl, error) {
		libraryPath, err := ml.fsToLibraryPathWithContext(fsPath, fsDirPath)
		if err != nil {
			return PathUrl{}, err
		}
		return PathUrl{
			Name: name,
			Url:  ml.libraryToUrlPath(libraryPath),
		}, nil
	}

	type Result struct {
		Dirs      []PathUrl `json:"dirs"`
		Playlists []PathUrl `json:"playlists"`
		Tracks    []PathUrl `json:"tracks"`
	}
	result := Result{
		Dirs:      make([]PathUrl, 0),
		Playlists: make([]PathUrl, 0),
		Tracks:    make([]PathUrl, 0),
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			slog.Error("entry.Info() failed", "entry", entry.Name(), "error", err)
			continue
		}
		mode := info.Mode()
		url := makeRelativePathUrl(entry.Name())

		if (mode & os.ModeSymlink) != 0 {
			linkPath := filepath.Join(fsDirPath, entry.Name())
			linkedPath, err := filepath.EvalSymlinks(linkPath)
			if err != nil {
				slog.Error("EvalSymlinks failed", "path", linkPath, "error", err)
				continue
			}

			linkedInfo, err := os.Stat(linkedPath)
			if err != nil {
				slog.Error("stat failed", "path", linkedPath, "error", err)
				continue
			}
			mode = linkedInfo.Mode()

			if mode.IsDir() {
				if absUrl, err := makeAbsolutePathUrl(entry.Name(), linkedPath); err == nil {
					url = absUrl
				}
			}
		}

		switch {
		case mode.IsDir():
			result.Dirs = append(result.Dirs, url)

		case mode.IsRegular():
			if reDirIgnore.MatchString(entry.Name()) && !reDirUnignore.MatchString(entry.Name()) {
				continue
			}
			if rePlaylist.MatchString(entry.Name()) {
				result.Playlists = append(result.Playlists, url)
			} else {
				result.Tracks = append(result.Tracks, url)
			}
		}
	}

	writeJson(w, result)
}
