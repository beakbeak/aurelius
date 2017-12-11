package database

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
)

func (db *Database) handlePlaylistRequestImpl(
	lines []string,
	w http.ResponseWriter,
	req *http.Request,
) error {
	if len(lines) < 1 {
		w.Write([]byte("null"))
		return nil
	}

	type Result struct {
		Pos  uint64 `json:"pos"`
		Path string `json:"path"`
	}

	result := Result{Pos: rand.Uint64() % uint64(len(lines))}
	result.Path = lines[result.Pos]

	resultBytes, err := json.Marshal(result)
	if err != nil {
		return err
	}
	w.Write(resultBytes)
	return nil
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
