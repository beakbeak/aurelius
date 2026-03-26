package media

import (
	"log/slog"
	"net/http"
)

// PlaylistTrack describes a track in a playlist.
type PlaylistTrack struct {
	Pos  int    `json:"pos"`
	Path string `json:"path"`
}

// Playlist describes a playlist.
type Playlist struct {
	Length int `json:"length"`
}

func (ml *Library) handleGetM3UPlaylist(
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

	writeJson(req.Context(), w, Playlist{Length: count})
}

func (ml *Library) handleGetM3UPlaylistTrack(
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

	writeJson(ctx, w, PlaylistTrack{
		Pos:  pos,
		Path: ml.libraryToUrlPath("tracks", trackPath),
	})
}

func (ml *Library) handleGetFavorites(
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

	writeJson(req.Context(), w, Playlist{Length: count})
}

func (ml *Library) handleGetFavoritesTrack(
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

	writeJson(ctx, w, PlaylistTrack{
		Pos:  pos,
		Path: ml.libraryToUrlPath("tracks", libraryPath),
	})
}
