package mediadb

import "embed"

//go:embed migrations/*.sql
var migrationFS embed.FS

// migrations is an ordered list of SQL migrations. Each entry corresponds to a
// database version. The database's PRAGMA user_version tracks which migrations
// have been applied.
var migrations = func() []string {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		panic("mediadb: reading embedded migrations: " + err.Error())
	}
	m := make([]string, len(entries))
	for i, e := range entries {
		data, err := migrationFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			panic("mediadb: reading migration " + e.Name() + ": " + err.Error())
		}
		m[i] = string(data)
	}
	return m
}()

// ReplayGain holds the four combinations of ReplayGain mode and clipping
// prevention.
type ReplayGain struct {
	Track       float64 `json:"track"`
	Album       float64 `json:"album"`
	TrackNoclip float64 `json:"trackNoclip"`
	AlbumNoclip float64 `json:"albumNoclip"`
}

// TrackMetadata holds audio properties stored in the metadata JSON column.
type TrackMetadata struct {
	Duration     float64     `json:"duration"`
	BitRate      int         `json:"bitRate"`
	SampleRate   uint        `json:"sampleRate"`
	SampleFormat string      `json:"sampleFormat"`
	ReplayGain   *ReplayGain `json:"replayGain,omitempty"`
}

// ImageInfo describes an image associated with a track. The binary data is
// stored in the images table and referenced by hash.
type ImageInfo struct {
	Hash     []byte
	MimeType string
	Size     int
}

// Track represents a row in the tracks table.
type Track struct {
	ID       int64
	Dir      string
	Name     string
	Mtime    int64
	Hash     []byte
	Tags     map[string]string
	Images   []ImageInfo
	Metadata TrackMetadata
}

// Dir represents a row in the dirs table.
type Dir struct {
	Path   string
	Parent string
}

// M3UPlaylist represents a row in the m3u_playlists table.
type M3UPlaylist struct {
	ID    int64
	Dir   string
	Name  string
	Mtime int64
}
