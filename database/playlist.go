package database

import (
	"net/http"
	"sb/aurelius/util"
	"strconv"
)

func (db *Database) handlePlaylistRequestImpl(
	lines []string,
	w http.ResponseWriter,
	req *http.Request,
) error {
	query := req.URL.Query()
	if posStr, ok := query["pos"]; ok {
		if len(lines) < 1 {
			return util.WriteJson(w, nil)
		}

		pos64, err := strconv.ParseInt(posStr[0], 0, 0)
		if err != nil {
			util.Debug.Printf("failed to parse playlist position '%v': %v\n", posStr, err)
			return util.WriteJson(w, nil)
		}
		pos := int(pos64)

		if pos < 0 || pos >= len(lines) {
			return util.WriteJson(w, nil)
		}

		type Result struct {
			Pos  int    `json:"pos"`
			Path string `json:"path"`
		}

		return util.WriteJson(w, Result{
			Pos:  pos,
			Path: lines[pos],
		})
	} else {
		type Result struct {
			Length int `json:"length"`
		}

		return util.WriteJson(w, Result{
			Length: len(lines),
		})
	}
}

func (db *Database) handlePlaylistRequest(
	matches []string,
	w http.ResponseWriter,
	req *http.Request,
) {
	path := db.expandPath(matches[1])
	lines, err := db.playlistCache.Get(path)
	if err != nil {
		http.NotFound(w, req)
		util.Debug.Printf("failed to load '%v': %v", path, err)
		return
	}

	if err := db.handlePlaylistRequestImpl(lines, w, req); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		util.Debug.Printf("playlist request failed: %v\n", err)
	}
}
