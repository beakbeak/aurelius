package mediadb

import (
	"bufio"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/beakbeak/aurelius/internal/maputil"
	"github.com/beakbeak/aurelius/pkg/aurelib"
	"github.com/beakbeak/aurelius/pkg/fragment"
)

const (
	imageProcessingLogInterval = 5 * time.Second
)

// FileInfo represents a file found on the filesystem during scanning.
type FileInfo struct {
	Dir   string
	Name  string
	Mtime int64
}

// HashedFileInfo is a FileInfo with a computed partial hash.
type HashedFileInfo struct {
	FileInfo
	Hash []byte
}

// Move represents a file that was moved/renamed.
type Move struct {
	OldDir   string
	OldName  string
	NewDir   string
	NewName  string
	NewMtime int64
}

// ChangeSet describes the differences between the filesystem and database.
type ChangeSet struct {
	Added   []HashedFileInfo
	Changed []FileInfo
	Removed []Track
	Moves   []Move

	AddedDirs   []Dir
	RemovedDirs []Dir

	AddedPlaylists   []FileInfo
	ChangedPlaylists []FileInfo
	RemovedPlaylists []M3UPlaylist
}

// ScannedPlaylist is a FileInfo enriched with resolved track library paths.
type ScannedPlaylist struct {
	FileInfo
	TrackPaths []string
}

// ScannedTrack is a FileInfo enriched with metadata read from the file.
type ScannedTrack struct {
	FileInfo
	Hash     []byte
	Tags     map[string]string
	Metadata TrackMetadata
}

// ScanResult contains the fully resolved changes, ready to apply.
type ScanResult struct {
	AddedTracks   []ScannedTrack
	ChangedTracks []ScannedTrack
	RemovedTracks []Track
	Moves         []Move
	AddedDirs     []Dir
	RemovedDirs   []Dir

	AddedPlaylists   []ScannedPlaylist
	ChangedPlaylists []ScannedPlaylist
	RemovedPlaylists []M3UPlaylist
}

// resolvedFragment holds the pre-resolved data for a fragment entry, used
// during metadata collection to construct the fragment audio source.
type resolvedFragment struct {
	SourceFSPath string
	SourceFile   string // source audio filename without dir
	Config       *FragmentConfig
	Index        int    // 1-based fragment index for this source file.
	hash         []byte // combined hash of config + source + index.
}

// WalkResult holds the filesystem state collected by walkFilesystem and
// enriched by expandFragments.
type WalkResult struct {
	Files      map[string]FileInfo // audio tracks keyed by library path
	Dirs       map[string]Dir      // directories keyed by library path
	Playlists  map[string]FileInfo // M3U playlists keyed by library path
	DirConfigs map[string]FileInfo // aurelius.yaml files keyed by directory

	// Fragments maps library paths (dir/syntheticName) to their resolved
	// fragment definitions. Populated during expandFragments and consumed
	// during collectMetadata.
	Fragments map[string]resolvedFragment
}

// Scanner coordinates filesystem scanning and database reconciliation.
type Scanner struct {
	db       *DB
	rootPath string
}

// NewScanner creates a new Scanner.
func NewScanner(db *DB, rootPath string) *Scanner {
	return &Scanner{db: db, rootPath: rootPath}
}

// fsPath returns the absolute filesystem path for a library path.
func (s *Scanner) fsPath(dir, name string) string {
	return filepath.Join(s.rootPath, filepath.FromSlash(dir), name)
}

// computeFragmentMtime computes a fragment's mtime from its config and source
// file mtimes.
func computeFragmentMtime(configMtime, sourceMtime int64) int64 {
	if sourceMtime > configMtime {
		return sourceMtime
	}
	return configMtime
}

// loadFragmentMtime computes a fragment's mtime by stat-ing the config and source
// files on disk. Returns 0 if the config file cannot be found.
func (s *Scanner) loadFragmentMtime(dir, sourceFile string) int64 {
	configInfo, err := os.Lstat(s.fsPath(dir, dirConfigName))
	if err != nil {
		return 0
	}
	var sourceMtime int64
	if sourceInfo, err := os.Lstat(s.fsPath(dir, sourceFile)); err == nil {
		sourceMtime = sourceInfo.ModTime().Unix()
	}
	return computeFragmentMtime(configInfo.ModTime().Unix(), sourceMtime)
}

// FullScan walks the entire filesystem, diffs against the DB, detects moves,
// collects metadata, and applies the result.
func (s *Scanner) FullScan() error {
	slog.Info("starting full media library scan", "root", s.rootPath)
	start := time.Now()

	// Phase 1: Walk filesystem.
	slog.Info("walking filesystem")
	wr, err := s.walkFilesystem()
	if err != nil {
		return fmt.Errorf("filesystem walk failed: %w", err)
	}
	slog.Info("filesystem walk complete",
		"files", len(wr.Files),
		"dirs", len(wr.Dirs),
		"playlists", len(wr.Playlists),
		"dirConfigs", len(wr.DirConfigs),
	)

	// Phase 1b: Expand fragments from aurelius.yaml files.
	s.expandFragments(wr)

	// Phase 2: Diff and detect moves.
	slog.Info("diffing against database")
	changes, err := s.diffAgainstDB(wr)
	if err != nil {
		return fmt.Errorf("diff failed: %w", err)
	}
	detectMoves(changes)
	if err := s.detectRevivals(changes); err != nil {
		return fmt.Errorf("revival detection failed: %w", err)
	}

	slog.Info("scan diff complete",
		"added", len(changes.Added),
		"changed", len(changes.Changed),
		"removed", len(changes.Removed),
		"moved", len(changes.Moves),
		"addedDirs", len(changes.AddedDirs),
		"removedDirs", len(changes.RemovedDirs),
		"addedPlaylists", len(changes.AddedPlaylists),
		"changedPlaylists", len(changes.ChangedPlaylists),
		"removedPlaylists", len(changes.RemovedPlaylists),
	)

	// Phase 3: Collect metadata.
	slog.Info("collecting metadata")
	result, err := s.collectMetadata(wr, changes)
	if err != nil {
		return fmt.Errorf("metadata collection failed: %w", err)
	}

	// Phase 4: Apply.
	slog.Info("applying changes to database")
	if err := s.Apply(wr, result); err != nil {
		return fmt.Errorf("apply failed: %w", err)
	}

	slog.Info("full scan complete", "duration", time.Since(start))
	return nil
}

// walkFilesystem walks the media library root and returns a WalkResult.
func (s *Scanner) walkFilesystem() (*WalkResult, error) {
	wr := &WalkResult{
		Files:      make(map[string]FileInfo),
		Dirs:       make(map[string]Dir),
		Playlists:  make(map[string]FileInfo),
		DirConfigs: make(map[string]FileInfo),
		Fragments:  make(map[string]resolvedFragment),
	}

	err := filepath.WalkDir(s.rootPath, func(fsPath string, d fs.DirEntry, err error) error {
		if err != nil {
			slog.Warn("walk error", "path", fsPath, "error", err)
			return nil
		}

		name := d.Name()

		// Skip hidden files/dirs and all symlinks.
		if strings.HasPrefix(name, ".") || d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(s.rootPath, fsPath)
		if err != nil {
			slog.Warn("failed to compute relative path", "path", fsPath, "error", err)
			return nil //nolint:nilerr // skip files we can't resolve
		}
		libraryPath := filepath.ToSlash(relPath)

		if d.IsDir() {
			libraryPath = CleanLibraryPath(libraryPath)
			if libraryPath == "" {
				return nil
			}
			wr.Dirs[libraryPath] = Dir{Path: libraryPath, Parent: CleanLibraryPath(path.Dir(libraryPath))}
			return nil
		}

		if !d.Type().IsRegular() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			slog.Warn("failed to get file info", "path", fsPath, "error", err)
			return nil
		}

		entry := FileInfo{
			Dir:   CleanLibraryPath(path.Dir(libraryPath)),
			Name:  name,
			Mtime: info.ModTime().Unix(),
		}

		switch GetFileType(name) {
		case FileTypeTrack:
			wr.Files[libraryPath] = entry
		case FileTypePlaylist:
			wr.Playlists[libraryPath] = entry
		case FileTypeDirConfig:
			wr.DirConfigs[entry.Dir] = entry
		case FileTypeImage, FileTypeIgnored:
		}

		return nil
	})

	return wr, err
}

// expandFragments parses aurelius.yaml files and adds synthetic fragment
// entries to wr.Files. It also populates wr.Fragments for use during
// metadata collection.
func (s *Scanner) expandFragments(wr *WalkResult) {
	for dir, configEntry := range wr.DirConfigs {
		configPath := s.fsPath(dir, dirConfigName)
		config, err := LoadDirConfig(configPath)
		if err != nil {
			slog.Warn("failed to parse dir config", "path", configPath, "error", err)
			continue
		}
		if len(config.Fragments) == 0 {
			continue
		}

		// Pre-compute the config file hash for fragment hashing.
		configHash, err := computeFullHash(configPath)
		if err != nil {
			slog.Warn("failed to hash dir config", "path", configPath, "error", err)
			continue
		}

		// Track per-source fragment numbering.
		sourceFragmentCount := make(map[string]int)

		for i := range config.Fragments {
			def := &config.Fragments[i]
			sourceFile := def.Source
			if _, ok := wr.Files[JoinLibraryPath(dir, sourceFile)]; !ok {
				slog.Warn("fragment source file not found",
					"dir", dir, "source", sourceFile)
				continue
			}

			sourceFragmentCount[sourceFile]++
			fragIdx := sourceFragmentCount[sourceFile]
			syntheticName := MakeFragmentName(sourceFile, fragIdx)
			libraryPath := JoinLibraryPath(dir, syntheticName)

			var sourceMtime int64
			if sourceEntry, ok := wr.Files[JoinLibraryPath(dir, sourceFile)]; ok {
				sourceMtime = sourceEntry.Mtime
			}
			mtime := computeFragmentMtime(configEntry.Mtime, sourceMtime)

			// Compute a combined hash for move detection.
			sourceHash, err := computePartialHash(s.fsPath(dir, sourceFile))
			if err != nil {
				slog.Warn("failed to hash fragment source", "dir", dir, "name", sourceFile, "error", err)
				continue
			}
			fragHash := computeFragmentHash(configHash, sourceHash, fragIdx)

			wr.Files[libraryPath] = FileInfo{
				Dir:   dir,
				Name:  syntheticName,
				Mtime: mtime,
			}

			wr.Fragments[libraryPath] = resolvedFragment{
				SourceFSPath: s.fsPath(dir, sourceFile),
				SourceFile:   sourceFile,
				Config:       def,
				Index:        fragIdx,
				hash:         fragHash,
			}
		}
	}

	if len(wr.Fragments) > 0 {
		slog.Info("expanded fragments from dir configs", "count", len(wr.Fragments))
	}
}

// computeFragmentHash computes a hash for a fragment entry by combining the
// config file hash, source file hash, and fragment index.
func computeFragmentHash(configHash, sourceHash []byte, fragmentIndex int) []byte {
	h := sha256.New()
	h.Write(configHash)
	h.Write(sourceHash)
	_ = binary.Write(h, binary.LittleEndian, int64(fragmentIndex))
	result := h.Sum(nil)
	return result
}

// diffAgainstDB compares filesystem state against the database.
func (s *Scanner) diffAgainstDB(wr *WalkResult) (*ChangeSet, error) {
	changes := &ChangeSet{}

	// Compare tracks.
	err := s.db.ForEachTrack(func(t *Track) error {
		key := JoinLibraryPath(t.Dir, t.Name)
		if fileInfo, ok := wr.Files[key]; ok {
			if fileInfo.Mtime != t.Mtime {
				changes.Changed = append(changes.Changed, fileInfo)
			}
			delete(wr.Files, key)
		} else {
			changes.Removed = append(changes.Removed, *t)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Remaining files are new.
	if len(wr.Files) > 0 {
		slog.Info("hashing new files", "count", len(wr.Files))
	}
	for key, entry := range wr.Files {
		var hash []byte
		if rf, ok := wr.Fragments[key]; ok {
			// Use pre-computed hash for fragment entries.
			hash = rf.hash
		} else {
			var err error
			hash, err = computePartialHash(s.fsPath(entry.Dir, entry.Name))
			if err != nil {
				slog.Warn("failed to hash file", "dir", entry.Dir, "name", entry.Name, "error", err)
				continue
			}
		}
		changes.Added = append(changes.Added, HashedFileInfo{
			FileInfo: entry,
			Hash:     hash,
		})
	}

	// Compare dirs.
	dbDirs, err := s.db.AllDirs()
	if err != nil {
		return nil, err
	}
	for dirPath, dir := range wr.Dirs {
		if _, ok := dbDirs[dirPath]; !ok {
			changes.AddedDirs = append(changes.AddedDirs, dir)
		}
		delete(dbDirs, dirPath)
	}
	for _, dir := range dbDirs {
		changes.RemovedDirs = append(changes.RemovedDirs, dir)
	}

	// Compare playlists.
	err = s.db.ForEachM3UPlaylist(func(p *M3UPlaylist) error {
		key := JoinLibraryPath(p.Dir, p.Name)
		if fileInfo, ok := wr.Playlists[key]; ok {
			if fileInfo.Mtime != p.Mtime {
				changes.ChangedPlaylists = append(changes.ChangedPlaylists, fileInfo)
			}
			delete(wr.Playlists, key)
		} else {
			changes.RemovedPlaylists = append(changes.RemovedPlaylists, *p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Remaining playlists are new.
	for _, entry := range wr.Playlists {
		changes.AddedPlaylists = append(changes.AddedPlaylists, entry)
	}

	return changes, nil
}

// detectMoves matches Added and Removed entries by hash to detect file moves.
func detectMoves(changes *ChangeSet) {
	if len(changes.Added) == 0 || len(changes.Removed) == 0 {
		return
	}

	// Build hash → removed track indices map.
	removedByHash := make(map[string][]int)
	for i, t := range changes.Removed {
		removedByHash[string(t.Hash)] = append(removedByHash[string(t.Hash)], i)
	}

	// Build hash → added entry indices map.
	addedByHash := make(map[string][]int)
	for i, a := range changes.Added {
		addedByHash[string(a.Hash)] = append(addedByHash[string(a.Hash)], i)
	}

	// Match added ↔ removed by hash.
	matchedAdded := make(map[int]bool)
	matchedRemoved := make(map[int]bool)

	for i, added := range changes.Added {
		hashKey := string(added.Hash)

		removedIndices := removedByHash[hashKey]
		if len(removedIndices) != 1 {
			continue
		}

		addedIndices := addedByHash[hashKey]
		if len(addedIndices) != 1 {
			continue
		}

		ri := removedIndices[0]
		if matchedRemoved[ri] {
			continue
		}

		changes.Moves = append(changes.Moves, Move{
			OldDir:   changes.Removed[ri].Dir,
			OldName:  changes.Removed[ri].Name,
			NewDir:   added.Dir,
			NewName:  added.Name,
			NewMtime: added.Mtime,
		})
		matchedAdded[i] = true
		matchedRemoved[ri] = true
	}

	// Remove matched entries from Added and Removed.
	if len(matchedAdded) > 0 {
		var newAdded []HashedFileInfo
		for i, a := range changes.Added {
			if !matchedAdded[i] {
				newAdded = append(newAdded, a)
			}
		}
		changes.Added = newAdded
	}

	if len(matchedRemoved) > 0 {
		var newRemoved []Track
		for i, r := range changes.Removed {
			if !matchedRemoved[i] {
				newRemoved = append(newRemoved, r)
			}
		}
		changes.Removed = newRemoved
	}
}

// detectRevivals matches Added entries against soft-deleted tracks in the
// database by hash. When a new file's hash matches a soft-deleted track, the
// entry is converted to a Move so that the original track ID (and its
// associated favorites, play history, etc.) is preserved.
func (s *Scanner) detectRevivals(changes *ChangeSet) error {
	if len(changes.Added) == 0 {
		return nil
	}

	matched := make(map[int]bool)
	for i, added := range changes.Added {
		deleted, err := s.db.SoftDeletedTrackByHash(added.Hash)
		if err != nil {
			return fmt.Errorf("failed to query soft-deleted track by hash: %w", err)
		}
		if deleted == nil {
			continue
		}

		changes.Moves = append(changes.Moves, Move{
			OldDir:   deleted.Dir,
			OldName:  deleted.Name,
			NewDir:   added.Dir,
			NewName:  added.Name,
			NewMtime: added.Mtime,
		})
		matched[i] = true
	}

	if len(matched) > 0 {
		slog.Info("revived soft-deleted tracks", "count", len(matched))
		var newAdded []HashedFileInfo
		for i, a := range changes.Added {
			if !matched[i] {
				newAdded = append(newAdded, a)
			}
		}
		changes.Added = newAdded
	}

	return nil
}

// collectMetadata reads metadata from files for added and changed entries.
func (s *Scanner) collectMetadata(wr *WalkResult, changes *ChangeSet) (*ScanResult, error) {
	result := &ScanResult{
		RemovedTracks: changes.Removed,
		Moves:         changes.Moves,
		AddedDirs:     changes.AddedDirs,
		RemovedDirs:   changes.RemovedDirs,
	}

	// Collect metadata for added tracks.
	for _, entry := range changes.Added {
		scanned, err := s.scanFile(wr, entry.FileInfo, entry.Hash)
		if err != nil {
			slog.Warn("failed to scan added file", "dir", entry.Dir, "name", entry.Name, "error", err)
			continue
		}
		result.AddedTracks = append(result.AddedTracks, *scanned)
	}

	// Collect metadata for changed tracks (also recompute hash).
	for _, entry := range changes.Changed {
		var hash []byte
		key := JoinLibraryPath(entry.Dir, entry.Name)
		if rf, ok := wr.Fragments[key]; ok {
			hash = rf.hash
		} else {
			var err error
			hash, err = computePartialHash(s.fsPath(entry.Dir, entry.Name))
			if err != nil {
				slog.Warn("failed to hash changed file", "dir", entry.Dir, "name", entry.Name, "error", err)
				continue
			}
		}
		scanned, err := s.scanFile(wr, entry, hash)
		if err != nil {
			slog.Warn("failed to scan changed file", "dir", entry.Dir, "name", entry.Name, "error", err)
			continue
		}
		result.ChangedTracks = append(result.ChangedTracks, *scanned)
	}

	// Parse added playlists.
	for _, entry := range changes.AddedPlaylists {
		lines, err := parseM3U(s.fsPath(entry.Dir, entry.Name))
		if err != nil {
			slog.Warn("failed to parse added playlist", "dir", entry.Dir, "name", entry.Name, "error", err)
			continue
		}
		result.AddedPlaylists = append(result.AddedPlaylists, ScannedPlaylist{
			FileInfo:   entry,
			TrackPaths: resolvePlaylistTracks(entry.Dir, lines),
		})
	}

	// Parse changed playlists.
	for _, entry := range changes.ChangedPlaylists {
		lines, err := parseM3U(s.fsPath(entry.Dir, entry.Name))
		if err != nil {
			slog.Warn("failed to parse changed playlist", "dir", entry.Dir, "name", entry.Name, "error", err)
			continue
		}
		result.ChangedPlaylists = append(result.ChangedPlaylists, ScannedPlaylist{
			FileInfo:   entry,
			TrackPaths: resolvePlaylistTracks(entry.Dir, lines),
		})
	}

	result.RemovedPlaylists = changes.RemovedPlaylists

	return result, nil
}

// parseM3U reads an .m3u file and returns the non-empty, non-comment lines.
func parseM3U(fsPath string) ([]string, error) {
	f, err := os.Open(fsPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}
	return lines, scanner.Err()
}

// resolvePlaylistTracks converts relative M3U lines to library paths.
func resolvePlaylistTracks(playlistDir string, lines []string) []string {
	paths := make([]string, 0, len(lines))
	for _, line := range lines {
		libraryPath := CleanLibraryPath(path.Join(playlistDir, line))
		paths = append(paths, libraryPath)
	}
	return paths
}

// resolveTrackFSPath returns the filesystem path for a track. For fragment
// entries, this returns the source audio file path.
func (s *Scanner) resolveTrackFSPath(wr *WalkResult, dir, name string) string {
	if wr != nil {
		if rf, ok := wr.Fragments[JoinLibraryPath(dir, name)]; ok {
			return rf.SourceFSPath
		}
	}
	return s.fsPath(dir, name)
}

// scanFile opens an audio file and extracts metadata.
func (s *Scanner) scanFile(wr *WalkResult, entry FileInfo, hash []byte) (*ScannedTrack, error) {
	libraryPath := JoinLibraryPath(entry.Dir, entry.Name)
	rf, isFragment := wr.Fragments[libraryPath]

	var src aurelib.Source
	var err error
	if isFragment {
		src, err = fragment.New(rf.SourceFSPath, rf.Config.Start, rf.Config.End)
	} else {
		src, err = aurelib.NewFileSource(s.fsPath(entry.Dir, entry.Name))
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open %s/%s: %w", entry.Dir, entry.Name, err)
	}
	defer src.Destroy()

	tags := maputil.LowerCaseKeys(src.Tags())

	// Apply fragment tag overrides.
	if isFragment {
		if rf.Config.Track != "" {
			tags["track"] = rf.Config.Track
		} else {
			tags["track"] = fmt.Sprintf("%v.%v", tags["track"], rf.Index)
		}
		if rf.Config.Artist != "" {
			tags["artist"] = rf.Config.Artist
		}
		if rf.Config.Title != "" {
			tags["title"] = rf.Config.Title
		}
		if rf.Config.Album != "" {
			tags["album"] = rf.Config.Album
		}
	}

	streamInfo := src.StreamInfo()

	metadata := TrackMetadata{
		Duration:     float64(src.Duration()) / float64(time.Second),
		Codec:        src.CodecName(),
		BitRate:      src.BitRate(),
		SampleRate:   streamInfo.SampleRate,
		SampleFormat: streamInfo.SampleFormat(),
	}

	// Store fragment info in metadata.
	if isFragment {
		metadata.Fragment = &Fragment{
			SourceFile: rf.SourceFile,
			Start:      rf.Config.Start.Seconds(),
			End:        rf.Config.End.Seconds(),
		}
	}

	// Collect all four ReplayGain combinations.
	rgTrack := src.ReplayGain(aurelib.ReplayGainTrack, true)
	rgAlbum := src.ReplayGain(aurelib.ReplayGainAlbum, true)
	rgTrackNoclip := src.ReplayGain(aurelib.ReplayGainTrack, false)
	rgAlbumNoclip := src.ReplayGain(aurelib.ReplayGainAlbum, false)

	// Only store ReplayGain if the source actually has RG data.
	// When there's no RG data, all values are 1.0.
	if rgTrack != 1.0 || rgAlbum != 1.0 || rgTrackNoclip != 1.0 || rgAlbumNoclip != 1.0 {
		metadata.ReplayGain = &ReplayGain{
			Track:       rgTrack,
			Album:       rgAlbum,
			TrackNoclip: rgTrackNoclip,
			AlbumNoclip: rgAlbumNoclip,
		}
	}

	return &ScannedTrack{
		FileInfo: entry,
		Hash:     hash,
		Tags:     tags,
		Metadata: metadata,
	}, nil
}

// marshalTrackJSON marshals the JSON fields of a ScannedTrack.
func marshalTrackJSON(t *ScannedTrack) (tagsJSON, metadataJSON string, err error) {
	tagsBytes, err := json.Marshal(t.Tags)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal tags: %w", err)
	}
	metadataBytes, err := json.Marshal(t.Metadata)
	if err != nil {
		return "", "", fmt.Errorf("failed to marshal metadata: %w", err)
	}
	return string(tagsBytes), string(metadataBytes), nil
}

// logImageProcessingError logs an error from image processing without
// stopping the scan.
func logImageProcessingError(context string, path string, err error) {
	slog.Warn("image processing failed",
		"context", context,
		"path", path,
		"error", err)
}

// trackImageWork identifies a track that needs image processing.
type trackImageWork struct {
	dir     string
	name    string
	trackID int64
}

// processAndInsertTrackImages processes images for the given tracks in parallel
// and inserts them within the provided transaction. Uses one goroutine per CPU
// core for image processing. Logs progress every 5 seconds if the operation
// takes longer than that.
//
// Uses a two-phase approach: phase 1 inserts all image data (via a streaming
// channel from workers), phase 2 creates track_images links (after all images
// are guaranteed to exist in the DB).
func (s *Scanner) processAndInsertTrackImages(wr *WalkResult, tx *sql.Tx, tracks []trackImageWork) error {
	if len(tracks) == 0 {
		return nil
	}

	insertImageStmt, err := tx.Prepare(
		`INSERT OR IGNORE INTO images (hash, original_hash, mime_type, data) VALUES (?, ?, ?, ?)`,
	)
	if err != nil {
		return err
	}
	defer insertImageStmt.Close()

	insertTrackImageStmt, err := tx.Prepare(
		`INSERT INTO track_images (track_id, position, image_hash) VALUES (?, ?, ?)`,
	)
	if err != nil {
		return err
	}
	defer insertTrackImageStmt.Close()

	deleteImagesStmt, err := tx.Prepare(`DELETE FROM track_images WHERE track_id = ?`)
	if err != nil {
		return err
	}
	defer deleteImagesStmt.Close()

	// Pre-load existing original_hash → hash mappings.
	slog.Info("loading image hash cache")
	cache := &imageHashCache{originalHashToProcessed: make(map[[32]byte][32]byte)}
	rows, err := tx.Query("SELECT original_hash, hash FROM images")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var origHash, hash []byte
			if err := rows.Scan(&origHash, &hash); err != nil {
				continue
			}
			var ok, hk [32]byte
			copy(ok[:], origHash)
			copy(hk[:], hash)
			cache.originalHashToProcessed[ok] = hk
		}
		if err := rows.Err(); err != nil {
			return err
		}
	}

	type trackImageMapping struct {
		trackID int64
		hashes  [][32]byte
	}

	imageCh := make(chan processedImage)
	var mappings []trackImageMapping
	var mappingsMu sync.Mutex

	// Group tracks by directory so that workers process all tracks in a
	// directory together, maximizing image cache hits for shared directory
	// images.
	type dirGroup struct {
		dir    string
		tracks []trackImageWork
	}
	groupsByDir := make(map[string]*dirGroup)
	for _, item := range tracks {
		g, ok := groupsByDir[item.dir]
		if !ok {
			g = &dirGroup{dir: item.dir}
			groupsByDir[item.dir] = g
		}
		g.tracks = append(g.tracks, item)
	}

	work := make(chan *dirGroup)
	var wg sync.WaitGroup

	// Log progress at regular intervals.
	var processed atomic.Int64
	total := int64(len(tracks))
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(imageProcessingLogInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				slog.Info("processing track images",
					"completed", processed.Load(),
					"total", total)
			}
		}
	}()

	slog.Info("processing track images", "tracks", total, "directories", len(groupsByDir))

	// Start workers that process all tracks in a directory group together.
	// This ensures directory images are only processed once per directory.
	numWorkers := runtime.NumCPU()
	for range numWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for group := range work {
				for _, item := range group.tracks {
					trackPath := s.resolveTrackFSPath(wr, item.dir, item.name)
					hashes := collectAndProcessTrackImages(trackPath, cache, imageCh)
					mappingsMu.Lock()
					mappings = append(mappings, trackImageMapping{trackID: item.trackID, hashes: hashes})
					mappingsMu.Unlock()
					processed.Add(1)
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(imageCh)
		close(done)
	}()

	go func() {
		for _, group := range groupsByDir {
			work <- group
		}
		close(work)
	}()

	// Phase 1: Insert all image data.
	for img := range imageCh {
		if _, err := insertImageStmt.Exec(img.hash[:], img.origHash[:], img.mimeType, img.data); err != nil {
			logImageProcessingError("insertImage", "image", err)
		}
	}

	// Phase 2: Create track_images links (all images now exist in DB).
	for _, m := range mappings {
		if _, err := deleteImagesStmt.Exec(m.trackID); err != nil {
			slog.Warn("failed to delete track images", "trackID", m.trackID, "error", err)
			continue
		}
		for pos, hash := range m.hashes {
			if _, err := insertTrackImageStmt.Exec(m.trackID, pos, hash[:]); err != nil {
				logImageProcessingError("insertTrackImage", fmt.Sprintf("trackID=%d", m.trackID), err)
			}
		}
	}

	return nil
}

// Apply writes a ScanResult to the database in a single transaction.
func (s *Scanner) Apply(wr *WalkResult, result *ScanResult) error {
	tx, err := s.db.db.Begin()
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	// Moves.
	if len(result.Moves) > 0 {
		// Hard-delete any soft-deleted track at the move destination to avoid
		// UNIQUE constraint violations.
		clearStmt, err := tx.Prepare(`DELETE FROM tracks_with_deletes WHERE dir = ? AND name = ? AND deleted = 1`)
		if err != nil {
			return err
		}
		defer clearStmt.Close()
		stmt, err := tx.Prepare(`UPDATE tracks_with_deletes SET dir = ?, name = ?, mtime = ?, deleted = 0 WHERE dir = ? AND name = ?`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, m := range result.Moves {
			// Skip the clear when source and destination are the same path
			// (revival in place) to avoid deleting the row we want to undelete.
			if m.OldDir != m.NewDir || m.OldName != m.NewName {
				if _, err := clearStmt.Exec(m.NewDir, m.NewName); err != nil {
					return fmt.Errorf("failed to clear soft-deleted track at move destination: %w", err)
				}
			}
			if _, err := stmt.Exec(m.NewDir, m.NewName, m.NewMtime, m.OldDir, m.OldName); err != nil {
				return fmt.Errorf("failed to apply move: %w", err)
			}
		}
	}

	var imageWork []trackImageWork

	// Changed.
	if len(result.ChangedTracks) > 0 {
		updateStmt, err := tx.Prepare(
			`UPDATE tracks_with_deletes SET mtime = ?, hash = ?, tags = ?, metadata = ?
			WHERE dir = ? AND name = ? RETURNING id`,
		)
		if err != nil {
			return err
		}
		defer updateStmt.Close()
		for i := range result.ChangedTracks {
			t := &result.ChangedTracks[i]
			tagsJSON, metadataJSON, err := marshalTrackJSON(t)
			if err != nil {
				return err
			}
			var trackID int64
			if err := updateStmt.QueryRow(t.Mtime, t.Hash, tagsJSON, metadataJSON, t.Dir, t.Name).Scan(&trackID); err != nil {
				return fmt.Errorf("failed to update track: %w", err)
			}
			imageWork = append(imageWork, trackImageWork{dir: t.Dir, name: t.Name, trackID: trackID})
		}
	}

	// Added.
	if len(result.AddedTracks) > 0 {
		stmt, err := tx.Prepare(
			`INSERT INTO tracks_with_deletes (dir, name, mtime, hash, tags, metadata, deleted)
			VALUES (?, ?, ?, ?, ?, ?, 0)
			ON CONFLICT(dir, name) DO UPDATE SET
				mtime = excluded.mtime,
				hash = excluded.hash,
				tags = excluded.tags,
				metadata = excluded.metadata,
				deleted = 0
			RETURNING id`,
		)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for i := range result.AddedTracks {
			t := &result.AddedTracks[i]
			tagsJSON, metadataJSON, err := marshalTrackJSON(t)
			if err != nil {
				return err
			}
			var trackID int64
			if err := stmt.QueryRow(t.Dir, t.Name, t.Mtime, t.Hash, tagsJSON, metadataJSON).Scan(&trackID); err != nil {
				return fmt.Errorf("failed to insert track: %w", err)
			}
			imageWork = append(imageWork, trackImageWork{dir: t.Dir, name: t.Name, trackID: trackID})
		}
	}

	// Process and insert images in parallel.
	slog.Info("starting image processing", "tracks", len(imageWork))
	if err := s.processAndInsertTrackImages(wr, tx, imageWork); err != nil {
		return err
	}

	// Removed.
	if len(result.RemovedTracks) > 0 {
		stmt, err := tx.Prepare(`UPDATE tracks_with_deletes SET deleted = 1 WHERE dir = ? AND name = ?`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, t := range result.RemovedTracks {
			if _, err := stmt.Exec(t.Dir, t.Name); err != nil {
				return fmt.Errorf("failed to soft-delete track: %w", err)
			}
		}
	}

	// Added dirs.
	if len(result.AddedDirs) > 0 {
		stmt, err := tx.Prepare(`INSERT OR IGNORE INTO dirs (path, parent) VALUES (?, ?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, d := range result.AddedDirs {
			if _, err := stmt.Exec(d.Path, d.Parent); err != nil {
				return fmt.Errorf("failed to insert dir: %w", err)
			}
		}
	}

	// Removed dirs.
	if len(result.RemovedDirs) > 0 {
		stmt, err := tx.Prepare(`DELETE FROM dirs WHERE path = ?`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, d := range result.RemovedDirs {
			if _, err := stmt.Exec(d.Path); err != nil {
				return fmt.Errorf("failed to delete dir: %w", err)
			}
		}
	}

	// Removed playlists.
	if len(result.RemovedPlaylists) > 0 {
		stmt, err := tx.Prepare(`DELETE FROM m3u_playlists WHERE dir = ? AND name = ?`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, p := range result.RemovedPlaylists {
			if _, err := stmt.Exec(p.Dir, p.Name); err != nil {
				return fmt.Errorf("failed to delete playlist: %w", err)
			}
		}
	}

	// Changed playlists.
	if len(result.ChangedPlaylists) > 0 {
		updateStmt, err := tx.Prepare(
			`UPDATE m3u_playlists SET mtime = ? WHERE dir = ? AND name = ?`,
		)
		if err != nil {
			return err
		}
		defer updateStmt.Close()
		deleteTracksStmt, err := tx.Prepare(
			`DELETE FROM m3u_playlist_tracks WHERE playlist_id = (
				SELECT id FROM m3u_playlists WHERE dir = ? AND name = ?
			)`,
		)
		if err != nil {
			return err
		}
		defer deleteTracksStmt.Close()
		insertTrackStmt, err := tx.Prepare(
			`INSERT INTO m3u_playlist_tracks (playlist_id, position, track_id)
			SELECT p.id, ?, t.id
			FROM m3u_playlists p, tracks t
			WHERE p.dir = ? AND p.name = ?
			  AND t.dir = ? AND t.name = ?`,
		)
		if err != nil {
			return err
		}
		defer insertTrackStmt.Close()
		for i := range result.ChangedPlaylists {
			p := &result.ChangedPlaylists[i]
			if _, err := updateStmt.Exec(p.Mtime, p.Dir, p.Name); err != nil {
				return fmt.Errorf("failed to update playlist: %w", err)
			}
			if _, err := deleteTracksStmt.Exec(p.Dir, p.Name); err != nil {
				return fmt.Errorf("failed to delete playlist tracks: %w", err)
			}
			for pos, trackPath := range p.TrackPaths {
				trackDir, trackName := SplitLibraryPath(trackPath)
				if _, err := insertTrackStmt.Exec(pos, p.Dir, p.Name, trackDir, trackName); err != nil {
					return fmt.Errorf("failed to insert playlist track: %w", err)
				}
			}
		}
	}

	// Added playlists.
	if len(result.AddedPlaylists) > 0 {
		insertPlaylistStmt, err := tx.Prepare(
			`INSERT INTO m3u_playlists (dir, name, mtime) VALUES (?, ?, ?)`,
		)
		if err != nil {
			return err
		}
		defer insertPlaylistStmt.Close()
		insertTrackStmt, err := tx.Prepare(
			`INSERT INTO m3u_playlist_tracks (playlist_id, position, track_id)
			SELECT ?, ?, t.id FROM tracks t WHERE t.dir = ? AND t.name = ?`,
		)
		if err != nil {
			return err
		}
		defer insertTrackStmt.Close()
		for i := range result.AddedPlaylists {
			p := &result.AddedPlaylists[i]
			res, err := insertPlaylistStmt.Exec(p.Dir, p.Name, p.Mtime)
			if err != nil {
				return fmt.Errorf("failed to insert playlist: %w", err)
			}
			playlistID, err := res.LastInsertId()
			if err != nil {
				return fmt.Errorf("failed to get playlist id: %w", err)
			}
			for pos, trackPath := range p.TrackPaths {
				trackDir, trackName := SplitLibraryPath(trackPath)
				if _, err := insertTrackStmt.Exec(playlistID, pos, trackDir, trackName); err != nil {
					return fmt.Errorf("failed to insert playlist track: %w", err)
				}
			}
		}
	}

	err = tx.Commit()
	if err == nil {
		committed = true
	}
	return err
}
