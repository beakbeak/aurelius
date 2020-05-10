package database

import (
	"net/http"
	"path"
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
		logger(LogDebug).Printf("failed to load '%v': %v", fsPath, err)
	}

	query := req.URL.Query()
	if posStr, ok := query["pos"]; ok {
		if len(lines) < 1 {
			writeJson(w, nil)
			return
		}

		pos64, err := strconv.ParseInt(posStr[0], 0, 0)
		if err != nil {
			logger(LogDebug).Printf("failed to parse playlist position '%v': %v\n", posStr, err)
			writeJson(w, nil)
			return
		}
		pos := int(pos64)

		if pos < 0 || pos >= len(lines) {
			writeJson(w, nil)
			return
		}

		type Result struct {
			Pos  int    `json:"pos"`
			Path string `json:"path"`
		}

		writeJson(w, Result{
			Pos:  pos,
			Path: db.toUrlPath(path.Join(path.Dir(dbPath), lines[pos])),
		})
	} else {
		type Result struct {
			Length int `json:"length"`
		}

		writeJson(w, Result{
			Length: len(lines),
		})
	}
}
