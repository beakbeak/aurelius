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
	path := db.expandPath(matches[1])

	infos, err := ioutil.ReadDir(path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		util.Debug.Printf("ReadDir failed: %v\n", err)
		return
	}

	type PathUrl struct {
		Name string
		Url  string
	}

	makePathUrl := func(path string) PathUrl {
		return PathUrl{
			Name: path,
			Url:  (&url.URL{Path: path}).String(),
		}
	}

	type TemplateData struct {
		Dirs      []PathUrl
		Playlists []PathUrl
		Tracks    []PathUrl
	}
	data := TemplateData{}

	for _, info := range infos {
		mode := info.Mode()
		if (mode & os.ModeSymlink) != 0 {
			linkName := filepath.Join(path, info.Name())
			info, err = os.Stat(linkName)
			if err != nil {
				util.Debug.Printf("stat '%v' failed: %v\n", linkName, err)
				continue
			}
			mode = info.Mode()
		}

		if mode.IsDir() {
			data.Dirs = append(data.Dirs, makePathUrl(info.Name()))
		} else if mode.IsRegular() {
			if reDirIgnore.MatchString(info.Name()) && !reDirUnignore.MatchString(info.Name()) {
				continue
			}
			if rePlaylist.MatchString(info.Name()) {
				data.Playlists = append(data.Playlists, makePathUrl(info.Name()))
			} else {
				data.Tracks = append(data.Tracks, makePathUrl(info.Name()))
			}
		}
	}

	if err := db.templateProxy.ExecuteTemplate(w, "db-dir.html", data); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		util.Debug.Printf("failed to execute template: %v\n", err)
	}
}
