package database

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
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
	reDirUnignore = regexp.MustCompile(`\.[aA][uU][rR]\.[tT][xX][tT]$`)
	rePlaylist = regexp.MustCompile(`\.[mM]3[uU]$`)
}

func (db *Database) handleDirRequest(
	matches []string,
	w http.ResponseWriter,
	_ *http.Request,
) {
	dbDirPath := matches[1]
	fsDirPath := db.toFileSystemPath(dbDirPath)

	infos, err := ioutil.ReadDir(fsDirPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		util.Debug.Printf("ReadDir failed: %v\n", err)
		return
	}

	type PathUrl struct {
		Name string
		Url  string
	}

	makePathUrl := func(name, urlPath string) PathUrl {
		return PathUrl{
			Name: name,
			Url:  (&url.URL{Path: urlPath}).String(),
		}
	}

	makeRelativePathUrl := func(name string) PathUrl {
		return makePathUrl(name, "./"+name)
	}

	makeAbsolutePathUrl := func(name, fsPath string) (PathUrl, error) {
		dbPath, err := db.toDatabasePath(fsPath)
		if err != nil {
			return PathUrl{}, err
		}
		return makePathUrl(name, db.toUrlPath(dbPath)), nil
	}

	type TemplateData struct {
		Dirs      []PathUrl
		Playlists []PathUrl
		Tracks    []PathUrl
	}
	data := TemplateData{}

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

			if absUrl, err := makeAbsolutePathUrl(info.Name(), linkedPath); err == nil {
				url = absUrl
			}
		}

		switch {
		case mode.IsDir():
			data.Dirs = append(data.Dirs, url)

		case mode.IsRegular():
			if reDirIgnore.MatchString(info.Name()) && !reDirUnignore.MatchString(info.Name()) {
				continue
			}
			if rePlaylist.MatchString(info.Name()) {
				data.Playlists = append(data.Playlists, url)
			} else {
				data.Tracks = append(data.Tracks, url)
			}
		}
	}

	if err := db.templateProxy.ExecuteTemplate(w, "db-dir.html", data); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		util.Debug.Printf("failed to execute template: %v\n", err)
	}
}
