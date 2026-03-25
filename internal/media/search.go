package media

import (
	"log/slog"
	"net/http"

	"github.com/beakbeak/aurelius/internal/mediadb"
)

// SearchResultJSON is the JSON representation of a single search result.
type SearchResultJSON struct {
	Path  string           `json:"path"`
	Type  string           `json:"type"`
	URL   string           `json:"url"`
	Track *TrackInfoResult `json:"track,omitempty"`
}

// SearchResponseJSON is the JSON representation of a search response.
type SearchResponseJSON struct {
	Results []SearchResultJSON `json:"results"`
}

func handleSearch(ml *Library, w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	query := r.URL.Query().Get("q")
	if query == "" {
		writeJson(ctx, w, &SearchResponseJSON{Results: []SearchResultJSON{}})
		return
	}

	results, err := ml.db.Search(query, 50)
	if err != nil {
		slog.ErrorContext(ctx, "search failed", "query", query, "error", err)
		http.Error(w, "Search failed", http.StatusInternalServerError)
		return
	}

	jsonResults := make([]SearchResultJSON, len(results.Results))
	for i, result := range results.Results {
		jr := SearchResultJSON{
			Path: result.Path,
			Type: result.Type,
		}

		if result.Type == mediadb.DocTypeTrack {
			jr.URL = ml.libraryToUrlPath("tracks", result.Path)
			track, err := ml.db.GetTrack(result.Path)
			if err != nil {
				slog.ErrorContext(ctx, "GetTrack failed for search result", "path", result.Path, "error", err)
			}
			if track != nil {
				favorite, err := ml.db.IsFavorite(result.Path)
				if err != nil {
					slog.ErrorContext(ctx, "IsFavorite failed for search result", "path", result.Path, "error", err)
				}
				info := ml.buildTrackInfo(track, favorite)
				jr.Track = &info
			}
		} else if result.Type == mediadb.DocTypeDirectory {
			jr.URL = ml.libraryToUrlPath("dirs", result.Path)
		}

		jsonResults[i] = jr
	}

	writeJson(ctx, w, &SearchResponseJSON{Results: jsonResults})
}
