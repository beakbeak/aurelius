package media

import (
	"log/slog"
	"net/http"
)

func (ml *Library) handleM3UPlaylistInfo(
	libraryPath string,
	w http.ResponseWriter,
	req *http.Request,
) {
	count, err := ml.db.GetM3UPlaylistTrackCount(libraryPath)
	if err != nil {
		slog.ErrorContext(req.Context(), "GetM3UPlaylistTrackCount failed", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	type Result struct {
		Length int `json:"length"`
	}
	writeJson(req.Context(), w, Result{Length: count})
}

func (ml *Library) handleM3UPlaylistTrack(
	libraryPath string,
	pos int,
	w http.ResponseWriter,
	req *http.Request,
) {
	ctx := req.Context()

	trackPath, err := ml.db.GetM3UPlaylistTrackAt(libraryPath, pos)
	if err != nil {
		slog.ErrorContext(ctx, "GetM3UPlaylistTrackAt failed", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if trackPath == "" {
		writeJson(ctx, w, nil)
		return
	}

	type Result struct {
		Pos  int    `json:"pos"`
		Path string `json:"path"`
	}
	writeJson(ctx, w, Result{
		Pos:  pos,
		Path: ml.libraryToUrlPath("tracks", trackPath),
	})
}

func (ml *Library) handleFavoritesInfo(
	w http.ResponseWriter,
	req *http.Request,
) {
	prefix := req.URL.Query().Get("prefix")
	count, err := ml.db.CountFavorites(prefix)
	if err != nil {
		slog.ErrorContext(req.Context(), "CountFavorites failed", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	type Result struct {
		Length int `json:"length"`
	}
	writeJson(req.Context(), w, Result{Length: count})
}

func (ml *Library) handleFavoritesTrack(
	pos int,
	w http.ResponseWriter,
	req *http.Request,
) {
	ctx := req.Context()
	prefix := req.URL.Query().Get("prefix")

	libraryPath, err := ml.db.GetFavoriteAt(pos, prefix)
	if err != nil {
		slog.ErrorContext(ctx, "GetFavoriteAt failed", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if libraryPath == "" {
		writeJson(ctx, w, nil)
		return
	}

	type Result struct {
		Pos  int    `json:"pos"`
		Path string `json:"path"`
	}
	writeJson(ctx, w, Result{
		Pos:  pos,
		Path: ml.libraryToUrlPath("tracks", libraryPath),
	})
}
