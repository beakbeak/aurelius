package mediadb

import (
	"fmt"
	"strings"
)

// SearchResult represents a single search result.
type SearchResult struct {
	Path string
	Type string
}

// SearchResponse represents the response from a search query.
type SearchResponse struct {
	Results []SearchResult
}

const (
	DocTypeTrack     = "track"
	DocTypeDirectory = "dir"
)

// Search performs a full-text search on library paths and returns matching
// tracks and directories.
func (db *DB) Search(query string, limit int) (*SearchResponse, error) {
	ftsQuery := buildFTSQuery(query)
	if ftsQuery == "" {
		return &SearchResponse{Results: []SearchResult{}}, nil
	}

	rows, err := db.db.Query(
		// Exclude tracks that are hidden because fragments reference them
		// as source files.
		`SELECT si.dir, si.name, si.type
		 FROM search_index si
		 WHERE search_index MATCH ?
		   AND (
		     si.type != 'track'
		     OR NOT EXISTS (
		       SELECT 1 FROM tracks frag
		       WHERE frag.dir = si.dir
		         AND json_extract(frag.metadata, '$.fragment.sourceFile') = si.name
		     )
		   )
		 ORDER BY rank
		 LIMIT ?`,
		ftsQuery, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search query failed: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var dir, name string
		var r SearchResult
		if err := rows.Scan(&dir, &name, &r.Type); err != nil {
			return nil, fmt.Errorf("failed to scan search result: %w", err)
		}
		if name == "" {
			r.Path = dir
		} else {
			r.Path = JoinLibraryPath(dir, name)
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if results == nil {
		results = []SearchResult{}
	}

	return &SearchResponse{
		Results: results,
	}, nil
}

// buildFTSQuery constructs an FTS5 trigram query from the user's search input.
// Terms are combined with AND. Trailing '*' is stripped since trigram matching
// is inherently substring-based.
func buildFTSQuery(query string) string {
	terms := strings.Fields(query)
	if len(terms) == 0 {
		return ""
	}

	var parts []string
	for _, term := range terms {
		term = strings.TrimRight(term, "*")
		if len(term) < 3 {
			continue // Trigram tokenizer requires at least 3 characters.
		}
		// Escape double quotes in the term.
		escaped := strings.ReplaceAll(term, `"`, `""`)
		parts = append(parts, fmt.Sprintf(`"%s"`, escaped))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " AND ")
}
