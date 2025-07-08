package search

// Result represents a single search result.
type Result struct {
	Path string `json:"path"`
	Type string `json:"type"`
	URL  string `json:"url"`
}

// Response represents the response from a search query.
type Response struct {
	Results []Result `json:"results"`
	Total   int      `json:"total"`
}

// document represents a document in the search index.
type document struct {
	ID             string `json:"-"`
	Path           string `json:"path"`
	PathWithoutExt string `json:"pathWithoutExt"`
	Type           string `json:"type"`
}

const (
	DocTypeTrack     = "track"
	DocTypeDirectory = "dir"
)
