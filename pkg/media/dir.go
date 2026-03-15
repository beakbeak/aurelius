package media

import (
	"context"
	"log/slog"
	"net/http"
	"path"
	"path/filepath"

	"github.com/beakbeak/aurelius/pkg/fragment"
)

func (ml *Library) handleDirInfo(
	ctx context.Context,
	dirLibraryPath string,
	w http.ResponseWriter,
) {
	dirLibraryPath = cleanLibraryPath(dirLibraryPath)

	subdirs, err := ml.db.GetSubdirs(dirLibraryPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		slog.ErrorContext(ctx, "GetSubdirs failed", "error", err)
		return
	}

	tracks, err := ml.db.GetTracksInDir(dirLibraryPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		slog.ErrorContext(ctx, "GetTracksInDir failed", "error", err)
		return
	}

	type PathUrl struct {
		Name     string `json:"name"`
		Url      string `json:"url"`
		Favorite bool   `json:"favorite,omitempty"`
	}

	type Result struct {
		Url       string    `json:"url"`
		TopLevel  string    `json:"topLevel"`
		Parent    string    `json:"parent"`
		Path      string    `json:"path"`
		Dirs      []PathUrl `json:"dirs"`
		Playlists []PathUrl `json:"playlists"`
		Tracks    []PathUrl `json:"tracks"`
	}
	result := Result{
		Url:       ml.libraryToUrlPath("dirs", dirLibraryPath),
		TopLevel:  ml.libraryToUrlPath("dirs", ""),
		Parent:    ml.libraryToUrlPath("dirs", cleanLibraryPath(path.Dir(dirLibraryPath))),
		Path:      dirLibraryPath,
		Dirs:      make([]PathUrl, 0, len(subdirs)),
		Playlists: make([]PathUrl, 0),
		Tracks:    make([]PathUrl, 0, len(tracks)),
	}

	for _, d := range subdirs {
		result.Dirs = append(result.Dirs, PathUrl{
			Name: filepath.Base(d.Path),
			Url:  ml.libraryToUrlPath("dirs", d.Path),
		})
	}

	// Query playlists from DB (only those with resolved tracks).
	dbPlaylists, err := ml.db.GetM3UPlaylistsInDir(dirLibraryPath)
	if err != nil {
		slog.ErrorContext(ctx, "GetM3UPlaylistsInDir failed", "error", err)
	}
	for _, p := range dbPlaylists {
		playlistPath := joinLibraryPath(p.Dir, p.Name)
		result.Playlists = append(result.Playlists, PathUrl{
			Name: p.Name,
			Url:  ml.libraryToUrlPath("playlists", playlistPath),
		})
	}

	// Build set of fragment source files to hide them from track listing.
	fragmentSourceFiles := make(map[string]bool)
	for _, t := range tracks {
		if fragment.IsFragment(t.Name) {
			if sourceFile := fragment.GetSourceFile(t.Name); sourceFile != "" {
				fragmentSourceFiles[sourceFile] = true
			}
		}
	}

	for _, t := range tracks {
		if fragmentSourceFiles[t.Name] {
			continue
		}
		trackPath := joinLibraryPath(t.Dir, t.Name)
		pu := PathUrl{
			Name: t.Name,
			Url:  ml.libraryToUrlPath("tracks", trackPath),
		}
		if fav, err := ml.db.IsFavorite(trackPath); err != nil {
			slog.ErrorContext(ctx, "IsFavorite failed", "path", trackPath, "error", err)
		} else {
			pu.Favorite = fav
		}
		result.Tracks = append(result.Tracks, pu)
	}

	writeJson(ctx, w, result)
}
