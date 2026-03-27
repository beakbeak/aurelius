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

type searchModifiers struct {
	dirsOnly      bool
	favoritesOnly bool
}

// Search performs a full-text search on library paths and returns matching
// tracks and directories.
func (db *DB) Search(query string, limit int) (*SearchResponse, error) {
	ftsQuery, mods := parseSearchQuery(query)
	if ftsQuery == "" || (mods.dirsOnly && mods.favoritesOnly) {
		return &SearchResponse{Results: []SearchResult{}}, nil
	}

	var where strings.Builder
	args := []any{ftsQuery}

	where.WriteString(`search_index MATCH ?`)

	// Exclude tracks that are hidden because fragments reference them
	// as source files.
	where.WriteString(`
		AND (
			si.type != 'track'
			OR NOT EXISTS (
				SELECT 1 FROM tracks frag
				WHERE frag.dir = si.dir
				AND json_extract(frag.metadata, '$.fragment.sourceFile') = si.name
			)
		)`)

	if mods.dirsOnly {
		where.WriteString(` AND si.type = 'dir'`)
	}
	if mods.favoritesOnly {
		where.WriteString(`
			AND EXISTS (
				SELECT 1 FROM tracks t
				JOIN favorites f ON f.track_id = t.id
				WHERE t.dir = si.dir AND t.name = si.name
			)`)
	}

	args = append(args, limit)

	rows, err := db.db.Query(
		fmt.Sprintf(
			`SELECT si.dir, si.name, si.type
			FROM search_index si
			WHERE %s
			ORDER BY rank
			LIMIT ?`, where.String()),
		args...,
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

// parseSearchQuery extracts search modifiers and constructs an FTS5 trigram
// query from the user's search input. The modifiers ".d" (directories only)
// and ".f" (favorites only) are recognized and stripped from the query. Remaining
// terms are combined with AND. Trailing '*' is stripped since trigram matching
// is inherently substring-based.
func parseSearchQuery(query string) (string, searchModifiers) {
	terms := strings.Fields(query)
	if len(terms) == 0 {
		return "", searchModifiers{}
	}

	var mods searchModifiers
	var parts []string
	for _, term := range terms {
		if term == ".d" {
			mods.dirsOnly = true
			continue
		}
		if term == ".f" {
			mods.favoritesOnly = true
			continue
		}
		term = strings.TrimRight(term, "*")
		if len(term) < 3 {
			continue // Trigram tokenizer requires at least 3 characters.
		}
		// Escape double quotes in the term.
		escaped := strings.ReplaceAll(term, `"`, `""`)
		parts = append(parts, fmt.Sprintf(`"%s"`, escaped))
	}
	if len(parts) == 0 {
		return "", mods
	}
	return strings.Join(parts, " AND "), mods
}
