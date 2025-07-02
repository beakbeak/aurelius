package media

import (
	"context"
	"log/slog"
	"net/http"
	"path"
	"strconv"
)

func (ml *Library) handlePlaylistRequest(
	ctx context.Context,
	libraryPath string,
	resource string,
	w http.ResponseWriter,
	req *http.Request,
) {
	fsPath := ml.libraryToFsPath(libraryPath)
	libraryDir := path.Dir(libraryPath)

	ml.handlePlaylistRequestImpl(ctx, fsPath, libraryDir, resource, w, req)
}

func (ml *Library) handleFavoritesRequest(
	ctx context.Context,
	resource string,
	w http.ResponseWriter,
	req *http.Request,
) {
	fsPath := ml.storageToFsPath(favoritesPath)
	libraryDir := "/"

	ml.handlePlaylistRequestImpl(ctx, fsPath, libraryDir, resource, w, req)
}

func (ml *Library) handlePlaylistRequestImpl(
	ctx context.Context,
	fsPath string,
	libraryDir string,
	resource string,
	w http.ResponseWriter,
	req *http.Request,
) {
	lines, err := ml.playlistCache.Get(fsPath)
	if err != nil {
		http.NotFound(w, req)
		slog.ErrorContext(ctx, "failed to load", "path", fsPath, "error", err)
	}

	switch resource {
	case "info":
		type Result struct {
			Length int `json:"length"`
		}

		writeJson(ctx, w, Result{
			Length: len(lines),
		})

	default: // element index
		if len(lines) < 1 {
			writeJson(ctx, w, nil)
			return
		}

		pos64, err := strconv.ParseInt(resource, 0, 0)
		if err != nil {
			slog.ErrorContext(ctx, "failed to parse playlist position", "position", resource, "error", err)
			writeJson(ctx, w, nil)
			return
		}
		pos := int(pos64)

		if pos < 0 || pos >= len(lines) {
			writeJson(ctx, w, nil)
			return
		}

		type Result struct {
			Pos  int    `json:"pos"`
			Path string `json:"path"`
		}

		writeJson(ctx, w, Result{
			Pos:  pos,
			Path: ml.libraryToUrlPath(path.Join(libraryDir, lines[pos])),
		})
	}
}
