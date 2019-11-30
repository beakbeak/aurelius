package database

import (
	"net/http"
	"sb/aurelius/util"
	"strconv"
)

func (db *Database) handlePlaylistRequest(
	dbPath string,
	w http.ResponseWriter,
	req *http.Request,
) {
	fsPath := db.toFileSystemPath(dbPath)
	lines, err := db.playlistCache.Get(fsPath)
	if err != nil {
		http.NotFound(w, req)
		util.Debug.Printf("failed to load '%v': %v", fsPath, err)
	}

	query := req.URL.Query()
	if posStr, ok := query["pos"]; ok {
		if len(lines) < 1 {
			util.WriteJson(w, nil)
			return
		}

		pos64, err := strconv.ParseInt(posStr[0], 0, 0)
		if err != nil {
			util.Debug.Printf("failed to parse playlist position '%v': %v\n", posStr, err)
			util.WriteJson(w, nil)
			return
		}
		pos := int(pos64)

		if pos < 0 || pos >= len(lines) {
			util.WriteJson(w, nil)
			return
		}

		type Result struct {
			Pos  int    `json:"pos"`
			Path string `json:"path"`
		}

		util.WriteJson(w, Result{
			Pos:  pos,
			Path: lines[pos],
		})
	} else {
		type Result struct {
			Length int `json:"length"`
		}

		util.WriteJson(w, Result{
			Length: len(lines),
		})
	}
}
