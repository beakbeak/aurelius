package database

import (
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sb/aurelius/util"
)

func (db *Database) handleDirRequest(
	matches []string,
	w http.ResponseWriter,
	req *http.Request,
) {
	path := db.expandPath(matches[1])

	infos, err := ioutil.ReadDir(path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		util.Debug.Printf("ReadDir failed: %v\n", err)
		return
	}

	type TemplateData struct {
		Dirs      []string
		Playlists []string
		Tracks    []string
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
			data.Dirs = append(data.Dirs, info.Name())
		} else if mode.IsRegular() {
			if db.reIgnore.FindStringIndex(info.Name()) != nil {
				continue
			}
			if db.rePlaylist.FindStringIndex(info.Name()) != nil {
				data.Playlists = append(data.Playlists, info.Name())
			} else {
				data.Tracks = append(data.Tracks, info.Name())
			}
		}
	}

	if err := db.templateProxy.ExecuteTemplate(w, "db-dir.html", data); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		util.Debug.Printf("failed to execute template: %v\n", err)
	}
}
