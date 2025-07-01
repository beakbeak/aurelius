package media

import (
	"log/slog"
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
	libraryDir := path.Dir(libraryPath)

	ml.handlePlaylistRequestImpl(fsPath, libraryDir, resource, w, req)
}

func (ml *Library) handleFavoritesRequest(
	resource string,
	w http.ResponseWriter,
	req *http.Request,
) {
	fsPath := ml.storageToFsPath(favoritesPath)
	libraryDir := "/"

	ml.handlePlaylistRequestImpl(fsPath, libraryDir, resource, w, req)
}

func (ml *Library) handlePlaylistRequestImpl(
	fsPath string,
	libraryDir string,
	resource string,
	w http.ResponseWriter,
	req *http.Request,
) {
	lines, err := ml.playlistCache.Get(fsPath)
	if err != nil {
		http.NotFound(w, req)
		slog.Error("failed to load", "path", fsPath, "error", err)
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
			slog.Error("failed to parse playlist position", "position", resource, "error", err)
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
			Path: ml.libraryToUrlPath(path.Join(libraryDir, lines[pos])),
		})
	}
}
