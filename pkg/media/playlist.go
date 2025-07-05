package media

import (
	"net/http"
	"path"

	"github.com/beakbeak/aurelius/pkg/textcache"
)

type playlist struct {
	fsPath, libraryDir string
	data               *textcache.CachedFile
}

func (ml *Library) loadFavorites() (*playlist, error) {
	out := playlist{
		fsPath:     ml.storageToFsPath(favoritesPath),
		libraryDir: "",
	}
	if err := out.load(ml); err != nil {
		return nil, err
	}
	return &out, nil
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
