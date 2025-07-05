package media

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"

	"github.com/beakbeak/aurelius/pkg/fragment"
)

var (
	reDirIgnore   = regexp.MustCompile(`(?i)\.(:?jpe?g|png|txt|log|cue|gif|pdf|sfv|nfo|bak)$`)
	reDirUnignore = regexp.MustCompile(`(?i)\.[0-9]+\.txt$`)
	rePlaylist    = regexp.MustCompile(`(?i)\.m3u$`)
)

func (ml *Library) handleDirInfo(
	ctx context.Context,
	dirLibraryPath string,
	w http.ResponseWriter,
) {
	fsDirPath := ml.libraryToFsPath(dirLibraryPath)

	entries, err := os.ReadDir(fsDirPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		slog.ErrorContext(ctx, "ReadDir failed", "error", err)
		return
	}

	type LibraryPath struct {
		Name string
		Path string
	}

	makeRelativeLibraryPath := func(name string) LibraryPath {
		return LibraryPath{
			Name: name,
			Path: path.Join(dirLibraryPath, name),
		}
	}

	makeAbsoluteLibraryPath := func(name, fsPath string) (LibraryPath, error) {
		libraryPath, err := ml.fsToLibraryPathWithContext(fsPath, fsDirPath)
		if err != nil {
			return LibraryPath{}, err
		}
		return LibraryPath{Name: name, Path: libraryPath}, nil
	}

	type PathUrl struct {
		Name string `json:"name"`
		Url  string `json:"url"`
	}

	makePathUrl := func(collection string, libraryPath LibraryPath) PathUrl {
		return PathUrl{
			Name: libraryPath.Name,
			Url:  ml.libraryToUrlPath(collection, libraryPath.Path),
		}
	}

	type Result struct {
		TopLevel  string    `json:"topLevel"`
		Parent    string    `json:"parent"`
		Path      string    `json:"path"`
		Dirs      []PathUrl `json:"dirs"`
		Playlists []PathUrl `json:"playlists"`
		Tracks    []PathUrl `json:"tracks"`
	}
	result := Result{
		TopLevel:  ml.libraryToUrlPath("dirs", ""),
		Parent:    ml.libraryToUrlPath("dirs", cleanLibraryPath(path.Dir(dirLibraryPath))),
		Path:      dirLibraryPath,
		Dirs:      make([]PathUrl, 0),
		Playlists: make([]PathUrl, 0),
		Tracks:    make([]PathUrl, 0),
	}

	fragmentSourceFiles := make(map[string]bool)
	for _, entry := range entries {
		if entry.Type().IsRegular() && fragment.IsFragment(entry.Name()) {
			if sourceFile := fragment.GetSourceFile(entry.Name()); sourceFile != "" {
				fragmentSourceFiles[sourceFile] = true
			}
		}
	}

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			slog.ErrorContext(ctx, "entry.Info() failed", "entry", entry.Name(), "error", err)
			continue
		}
		mode := info.Mode()
		entryLibraryPath := makeRelativeLibraryPath(entry.Name())

		if (mode & os.ModeSymlink) != 0 {
			linkPath := filepath.Join(fsDirPath, entry.Name())
			linkedPath, err := filepath.EvalSymlinks(linkPath)
			if err != nil {
				slog.ErrorContext(ctx, "EvalSymlinks failed", "path", linkPath, "error", err)
				continue
			}

			linkedInfo, err := os.Stat(linkedPath)
			if err != nil {
				slog.ErrorContext(ctx, "stat failed", "path", linkedPath, "error", err)
				continue
			}
			mode = linkedInfo.Mode()

			if mode.IsDir() {
				if absPath, err := makeAbsoluteLibraryPath(entry.Name(), linkedPath); err == nil {
					entryLibraryPath = absPath
				}
			}
		}

		switch {
		case mode.IsDir():
			result.Dirs = append(result.Dirs, makePathUrl("dirs", entryLibraryPath))

		case mode.IsRegular():
			if reDirIgnore.MatchString(entry.Name()) && !reDirUnignore.MatchString(entry.Name()) {
				continue
			}
			if fragmentSourceFiles[entry.Name()] {
				continue
			}
			if rePlaylist.MatchString(entry.Name()) {
				result.Playlists = append(result.Playlists, makePathUrl("playlists", entryLibraryPath))
			} else {
				result.Tracks = append(result.Tracks, makePathUrl("tracks", entryLibraryPath))
			}
		}
	}

	writeJson(ctx, w, result)
}
