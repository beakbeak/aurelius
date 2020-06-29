package media

import (
	"net/http"
	"path"
	"strconv"
)

func (ml *Library) handlePlaylistRequest(
	libraryPath string,
	resource string,
	w http.ResponseWriter,
	req *http.Request,
) {
	fsPath := ml.libraryToFsPath(libraryPath)
	lines, err := ml.playlistCache.Get(fsPath)
	if err != nil {
		http.NotFound(w, req)
		log.Printf("failed to load '%v': %v", fsPath, err)
	}

	switch resource {
	case "info":
		type Result struct {
			Length int `json:"length"`
		}

		writeJson(w, Result{
			Length: len(lines),
		})

	default: // element index
		if len(lines) < 1 {
			writeJson(w, nil)
			return
		}

		pos64, err := strconv.ParseInt(resource, 0, 0)
		if err != nil {
			log.Printf("failed to parse playlist position '%v': %v\n", resource, err)
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
			Path: ml.libraryToUrlPath(path.Join(path.Dir(libraryPath), lines[pos])),
		})
	}
}
