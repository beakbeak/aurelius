package media

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/beakbeak/aurelius/pkg/fragment"
	"github.com/beakbeak/aurelius/pkg/mediadb"
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

	// Discover playlists from the filesystem (not stored in DB).
	dirFsPath := ml.libraryToFsPath(dirLibraryPath)
	if entries, err := os.ReadDir(dirFsPath); err == nil {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			if !entry.Type().IsRegular() {
				continue
			}
			if mediadb.GetFileType(entry.Name()) == mediadb.FileTypePlaylist {
				playlistPath := path.Join(dirLibraryPath, entry.Name())
				result.Playlists = append(result.Playlists, PathUrl{
					Name: entry.Name(),
					Url:  ml.libraryToUrlPath("playlists", playlistPath),
				})
			}
		}
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
