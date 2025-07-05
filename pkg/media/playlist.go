package media

import (
	"net/http"
	"path"
)

type playlist struct {
	fsPath, libraryDir string
	lines              []string
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
	file, err := ml.playlistCache.Get(p.fsPath)
	if err != nil {
		return err
	}
	p.lines = file.Lines()
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

	writeJson(req.Context(), w, Result{
		Length: len(p.lines),
	})
}

func (ml *Library) handlePlaylistTrack(
	p *playlist,
	pos int,
	w http.ResponseWriter,
	req *http.Request,
) {
	ctx := req.Context()
	if len(p.lines) < 1 {
		writeJson(ctx, w, nil)
		return
	}
	if pos < 0 || pos >= len(p.lines) {
		writeJson(ctx, w, nil)
		return
	}

	type Result struct {
		Pos  int    `json:"pos"`
		Path string `json:"path"`
	}

	writeJson(ctx, w, Result{
		Pos:  pos,
		Path: ml.libraryToUrlPath("tracks", path.Join(p.libraryDir, p.lines[pos])),
	})
}
