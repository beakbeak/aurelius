package media

import (
	"log/slog"
	"net/http"
	"path"

	"github.com/beakbeak/aurelius/pkg/textcache"
)

type playlist struct {
	fsPath, libraryDir string
	data               *textcache.CachedFile
}

func (ml *Library) loadPlaylist(libraryPath string) (*playlist, error) {
	out := playlist{
		fsPath:     ml.libraryToFsPath(libraryPath),
		libraryDir: path.Dir(libraryPath),
	}
	if err := out.load(ml); err != nil {
		return nil, err
	}
	return &out, nil
}

func (p *playlist) load(ml *Library) error {
	data, err := ml.playlistCache.Get(p.fsPath)
	if err != nil {
		return err
	}
	p.data = data
	return nil
}

func (ml *Library) handlePlaylistInfo(
	p *playlist,
	w http.ResponseWriter,
	req *http.Request,
) {
	type Result struct {
		Length int `json:"length"`
	}

	var lines []string
	if prefix := req.URL.Query().Get("prefix"); prefix != "" {
		lines = p.data.LinesWithPrefix(prefix)
	} else {
		lines = p.data.Lines()
	}

	writeJson(req.Context(), w, Result{
		Length: len(lines),
	})
}

func (ml *Library) handlePlaylistTrack(
	p *playlist,
	pos int,
	w http.ResponseWriter,
	req *http.Request,
) {
	ctx := req.Context()

	var lines []string
	if prefix := req.URL.Query().Get("prefix"); prefix != "" {
		lines = p.data.LinesWithPrefix(prefix)
	} else {
		lines = p.data.Lines()
	}

	if len(lines) < 1 {
		writeJson(ctx, w, nil)
		return
	}
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
		Path: ml.libraryToUrlPath("tracks", path.Join(p.libraryDir, lines[pos])),
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
