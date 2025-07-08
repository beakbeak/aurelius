package search

import (
	"io/fs"
	"log/slog"
	"path/filepath"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/analysis/analyzer/simple"
)

// FileFilter returns true if a file should be indexed.
type FileFilter func(path string, d fs.DirEntry) bool

// Index manages the search index for the media library.
type Index struct {
	index      bleve.Index
	rootPath   string
	fileFilter FileFilter
}

// NewIndex creates a new search index.
func NewIndex(rootPath string, fileFilter FileFilter) (*Index, error) {
	indexMapping := bleve.NewIndexMapping()

	docMapping := bleve.NewDocumentMapping()
	docMapping.DefaultAnalyzer = simple.Name

	pathField := bleve.NewTextFieldMapping()
	pathField.Store = true
	pathField.Index = false
	docMapping.AddFieldMappingsAt("path", pathField)

	pathWithoutExtField := bleve.NewTextFieldMapping()
	pathWithoutExtField.Store = false
	pathWithoutExtField.Index = true
	pathWithoutExtField.Analyzer = simple.Name
	docMapping.AddFieldMappingsAt("pathWithoutExt", pathWithoutExtField)

	typeField := bleve.NewTextFieldMapping()
	typeField.Store = true
	typeField.Index = false
	docMapping.AddFieldMappingsAt("type", typeField)

	indexMapping.AddDocumentMapping("_default", docMapping)

	// Create in-memory index
	index, err := bleve.NewMemOnly(indexMapping)
	if err != nil {
		return nil, err
	}

	return &Index{
		index:      index,
		rootPath:   rootPath,
		fileFilter: fileFilter,
	}, nil
}

// BuildIndex walks the directory tree and builds the search index.
func (si *Index) BuildIndex() error {
	slog.Info("building search index", "rootPath", si.rootPath)

	err := filepath.WalkDir(si.rootPath, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			slog.Error("error walking directory", "path", path, "error", err)
			return nil // Continue walking
		}

		// Skip hidden files and directories
		if strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Apply file filter
		if si.fileFilter != nil && !si.fileFilter(path, entry) {
			return nil
		}

		// Get relative path from root
		relPath, err := filepath.Rel(si.rootPath, path)
		if err != nil {
			slog.Error("failed to get relative path", "path", path, "error", err)
			return nil
		}

		// Convert to library path (use forward slashes)
		slashPath := filepath.ToSlash(relPath)

		var doc document
		if entry.IsDir() {
			doc = document{
				ID:             slashPath,
				Path:           slashPath,
				PathWithoutExt: slashPath, // Full path for directories
				Type:           DocTypeDirectory,
			}
		} else {
			pathWithoutExt := strings.TrimSuffix(slashPath, filepath.Ext(slashPath))
			doc = document{
				ID:             slashPath,
				Path:           slashPath,
				PathWithoutExt: pathWithoutExt, // Remove extension for tracks
				Type:           DocTypeTrack,
			}
		}

		if err := si.index.Index(doc.ID, doc); err != nil {
			slog.Error("failed to index document", "id", doc.ID, "error", err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	// Get index stats
	docCount, err := si.index.DocCount()
	if err != nil {
		slog.Error("failed to get doc count", "error", err)
	} else {
		slog.Info("search index built successfully", "documents", docCount)
	}

	return nil
}

// Search performs a search query and returns results.
func (si *Index) Search(query string, limit int) (*Response, error) {
	if query == "" {
		return &Response{Results: []Result{}, Total: 0}, nil
	}

	// Create a disjunction query that combines exact matches with fuzzy matches
	disjunctionQuery := bleve.NewDisjunctionQuery()

	// Add exact match query (higher relevance)
	exactQuery := bleve.NewQueryStringQuery(query)
	disjunctionQuery.AddQuery(exactQuery)

	// Add fuzzy query for approximate matches
	// Split query into terms and create fuzzy queries for each
	terms := strings.Fields(query)
	for _, term := range terms {
		fuzzyQuery := bleve.NewFuzzyQuery(term)
		fuzzyQuery.SetFuzziness(2) // Allow up to 2 character differences
		disjunctionQuery.AddQuery(fuzzyQuery)
	}

	// Create search request
	searchRequest := bleve.NewSearchRequestOptions(disjunctionQuery, limit, 0, false)
	searchRequest.Fields = []string{"path", "type"}

	// Execute search
	searchResult, err := si.index.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	// Convert results
	results := make([]Result, 0, len(searchResult.Hits))
	for _, hit := range searchResult.Hits {
		path, _ := hit.Fields["path"].(string)
		docType, _ := hit.Fields["type"].(string)

		results = append(results, Result{
			Path: path,
			Type: docType,
			URL:  "", // Will be set by the caller
		})
	}

	return &Response{
		Results: results,
		Total:   int(searchResult.Total),
	}, nil
}

// Close closes the search index.
func (si *Index) Close() error {
	return si.index.Close()
}
