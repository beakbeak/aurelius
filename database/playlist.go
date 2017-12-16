package database

import (
	"fmt"
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
	w http.ResponseWriter,
	req *http.Request,
) (handled bool, _ error) {
	groups := db.rePlaylistPath.FindStringSubmatch(req.URL.Path)
	if groups == nil {
		return false, nil
	}

	path := db.expandPath(groups[1])
	lines, err := db.playlistCache.Get(path)
	if err != nil {
		return false, fmt.Errorf("failed to load '%v': %v", path, err)
	}

	return true, db.handlePlaylistRequestImpl(lines, w, req)
}
