package database

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sb/aurelius/util"
)

var (
	reDirIgnore   *regexp.Regexp
	reDirUnignore *regexp.Regexp

	rePlaylist *regexp.Regexp
)

func init() {
	reDirIgnore = regexp.MustCompile(`(?i)\.(:?jpe?g|png|txt|log|cue|gif|pdf|sfv|nfo|bak)$`)
	reDirUnignore = regexp.MustCompile(`(?i)\.[0-9]+\.txt$`)
	rePlaylist = regexp.MustCompile(`(?i)\.m3u$`)
}

func (db *Database) handleDirRequest(
	dbDirPath string,
	w http.ResponseWriter,
	req *http.Request,
) {
	query := req.URL.Query()

	if _, ok := query["info"]; ok {
		db.handleDirInfoRequest(dbDirPath, w)
		return
	}

	http.ServeFile(w, req, db.toHtmlPath("main.html"))
}

func (db *Database) handleDirInfoRequest(
	dbDirPath string,
	w http.ResponseWriter,
) {
	fsDirPath := db.toFileSystemPath(dbDirPath)

	infos, err := ioutil.ReadDir(fsDirPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		util.Debug.Printf("ReadDir failed: %v\n", err)
		return
	}

	type PathUrl struct {
		Name string `json:"name"`
		Url  string `json:"url"`
	}

	makePathUrl := func(name, urlPath string) PathUrl {
		return PathUrl{
			Name: name,
			Url:  (&url.URL{Path: urlPath}).String(),
		}
	}

	makeRelativePathUrl := func(name string) PathUrl {
		return makePathUrl(name, db.toUrlPath(path.Join(dbDirPath, name)))
	}

	makeAbsolutePathUrl := func(name, fsPath string) (PathUrl, error) {
		dbPath, err := db.toDatabasePathWithContext(fsPath, fsDirPath)
		if err != nil {
			return PathUrl{}, err
		}
		return makePathUrl(name, db.toUrlPath(dbPath)), nil
	}

	type Result struct {
		Dirs      []PathUrl `json:"dirs"`
		Playlists []PathUrl `json:"playlists"`
		Tracks    []PathUrl `json:"tracks"`
	}
	result := Result{}

	for _, info := range infos {
		mode := info.Mode()
		url := makeRelativePathUrl(info.Name())

		if (mode & os.ModeSymlink) != 0 {
			linkPath := filepath.Join(fsDirPath, info.Name())
			linkedPath, err := filepath.EvalSymlinks(linkPath)
			if err != nil {
				util.Debug.Printf("EvalSymlinks(%v) failed: %v\n", linkPath, err)
				continue
			}

			linkedInfo, err := os.Stat(linkedPath)
			if err != nil {
				util.Debug.Printf("stat '%v' failed: %v\n", linkedPath, err)
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

	util.WriteJson(w, result)
}
