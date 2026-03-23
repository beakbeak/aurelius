package media

import (
	"context"
	"log/slog"
	"net/http"
	"path"
	"path/filepath"

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
		Name string `json:"name"`
		Url  string `json:"url"`
	}

	type Result struct {
		Url       string            `json:"url"`
		TopLevel  string            `json:"topLevel"`
		Parent    string            `json:"parent"`
		Path      string            `json:"path"`
		Dirs      []PathUrl         `json:"dirs"`
		Playlists []PathUrl         `json:"playlists"`
		Tracks    []trackInfoResult `json:"tracks"`
	}
	result := Result{
		Url:       ml.libraryToUrlPath("dirs", dirLibraryPath),
		TopLevel:  ml.libraryToUrlPath("dirs", ""),
		Parent:    ml.libraryToUrlPath("dirs", cleanLibraryPath(path.Dir(dirLibraryPath))),
		Path:      dirLibraryPath,
		Dirs:      make([]PathUrl, 0, len(subdirs)),
		Playlists: make([]PathUrl, 0),
		Tracks:    make([]trackInfoResult, 0, len(tracks)),
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

	// Bulk-load images and favorites for all tracks in the directory.
	trackImages, err := ml.db.GetTrackImagesInDir(dirLibraryPath)
	if err != nil {
		slog.ErrorContext(ctx, "GetTrackImagesInDir failed", "error", err)
	}
	favorites, err := ml.db.GetFavoritesInDir(dirLibraryPath)
	if err != nil {
		slog.ErrorContext(ctx, "GetFavoritesInDir failed", "error", err)
	}

	// Build set of fragment source files to hide them from track listing.
	fragmentSourceFiles := make(map[string]bool)
	for _, t := range tracks {
		if t.Metadata.Fragment != nil {
			fragmentSourceFiles[t.Metadata.Fragment.SourceFile] = true
		}
	}

	for _, t := range tracks {
		if fragmentSourceFiles[t.Name] {
			continue
		}
		t.Images = trackImages[t.ID]
		result.Tracks = append(result.Tracks, ml.buildTrackInfo(&t, favorites[t.ID]))
	}

	writeJson(ctx, w, result)
}
