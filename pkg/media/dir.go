package media

import (
	"io/ioutil"
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

	infos, err := ioutil.ReadDir(fsDirPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("ReadDir failed: %v\n", err)
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

	for _, info := range infos {
		mode := info.Mode()
		url := makeRelativePathUrl(info.Name())

		if (mode & os.ModeSymlink) != 0 {
			linkPath := filepath.Join(fsDirPath, info.Name())
			linkedPath, err := filepath.EvalSymlinks(linkPath)
			if err != nil {
				log.Printf("EvalSymlinks(%v) failed: %v\n", linkPath, err)
				continue
			}

			linkedInfo, err := os.Stat(linkedPath)
			if err != nil {
				log.Printf("stat '%v' failed: %v\n", linkedPath, err)
				continue
			}
			mode = linkedInfo.Mode()

			if mode.IsDir() {
				if absUrl, err := makeAbsolutePathUrl(info.Name(), linkedPath); err == nil {
					url = absUrl
				}
			}
		}

		switch {
		case mode.IsDir():
			result.Dirs = append(result.Dirs, url)

		case mode.IsRegular():
			if reDirIgnore.MatchString(info.Name()) && !reDirUnignore.MatchString(info.Name()) {
				continue
			}
			if rePlaylist.MatchString(info.Name()) {
				result.Playlists = append(result.Playlists, url)
			} else {
				result.Tracks = append(result.Tracks, url)
			}
		}
	}

	writeJson(w, result)
}
