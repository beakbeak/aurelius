package mediadb

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/beakbeak/aurelius/internal/maputil"
	"github.com/beakbeak/aurelius/pkg/aurelib"
	"github.com/beakbeak/aurelius/pkg/fragment"
)

// FSEntry represents a file found on the filesystem during scanning.
type FSEntry struct {
	Dir   string
	Name  string
	Mtime int64
}

// HashedFSEntry is an FSEntry with a computed partial hash.
type HashedFSEntry struct {
	FSEntry
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
	Added   []HashedFSEntry
	Changed []FSEntry
	Removed []Track
	Moves   []Move

	AddedDirs   []Dir
	RemovedDirs []Dir
}

// ScannedTrack is an FSEntry enriched with metadata read from the file.
type ScannedTrack struct {
	FSEntry
	Hash           []byte
	Tags           map[string]string
	AttachedImages []AttachedImageInfo
	Metadata       TrackMetadata
}

// ScanResult contains the fully resolved changes, ready to apply.
type ScanResult struct {
	AddedTracks   []ScannedTrack
	ChangedTracks []ScannedTrack
	RemovedTracks []Track
	Moves         []Move
	AddedDirs     []Dir
	RemovedDirs   []Dir
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

// FullScan walks the entire filesystem, diffs against the DB, detects moves,
// collects metadata, and applies the result.
func (s *Scanner) FullScan() error {
	slog.Info("starting full media library scan", "root", s.rootPath)
	start := time.Now()

	// Phase 1: Walk filesystem.
	fsFiles, fsDirs, err := s.walkFilesystem()
	if err != nil {
		return fmt.Errorf("filesystem walk failed: %w", err)
	}

	// Phase 2: Diff and detect moves.
	changes, err := s.diffAgainstDB(fsFiles, fsDirs)
	if err != nil {
		return fmt.Errorf("diff failed: %w", err)
	}
	detectMoves(changes)

	slog.Info("scan diff complete",
		"added", len(changes.Added),
		"changed", len(changes.Changed),
		"removed", len(changes.Removed),
		"moved", len(changes.Moves),
		"addedDirs", len(changes.AddedDirs),
		"removedDirs", len(changes.RemovedDirs),
	)

	// Phase 3: Collect metadata.
	result, err := s.collectMetadata(changes)
	if err != nil {
		return fmt.Errorf("metadata collection failed: %w", err)
	}

	// Phase 4: Apply.
	if err := s.Apply(result); err != nil {
		return fmt.Errorf("apply failed: %w", err)
	}

	slog.Info("full scan complete", "duration", time.Since(start))
	return nil
}

// walkFilesystem walks the media library root and returns maps of files and directories.
func (s *Scanner) walkFilesystem() (map[string]FSEntry, map[string]Dir, error) {
	fsFiles := make(map[string]FSEntry)
	fsDirs := make(map[string]Dir)

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
			fsDirs[libraryPath] = Dir{Path: libraryPath, Parent: CleanLibraryPath(path.Dir(libraryPath))}
			return nil
		}

		if !d.Type().IsRegular() {
			return nil
		}

		if GetFileType(name) != FileTypeTrack {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			slog.Warn("failed to get file info", "path", fsPath, "error", err)
			return nil
		}

		fsFiles[libraryPath] = FSEntry{
			Dir:   CleanLibraryPath(path.Dir(libraryPath)),
			Name:  name,
			Mtime: info.ModTime().Unix(),
		}

		return nil
	})

	return fsFiles, fsDirs, err
}

// diffAgainstDB compares filesystem state against the database.
func (s *Scanner) diffAgainstDB(fsFiles map[string]FSEntry, fsDirs map[string]Dir) (*ChangeSet, error) {
	changes := &ChangeSet{}

	// Compare tracks.
	err := s.db.ForEachTrack(func(t *Track) error {
		key := JoinLibraryPath(t.Dir, t.Name)
		if fsEntry, ok := fsFiles[key]; ok {
			if fsEntry.Mtime != t.Mtime {
				changes.Changed = append(changes.Changed, fsEntry)
			}
			delete(fsFiles, key)
		} else {
			changes.Removed = append(changes.Removed, *t)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Remaining fsFiles are new.
	for _, entry := range fsFiles {
		hash, err := computePartialHash(s.fsPath(entry.Dir, entry.Name))
		if err != nil {
			slog.Warn("failed to hash file", "dir", entry.Dir, "name", entry.Name, "error", err)
			continue
		}
		changes.Added = append(changes.Added, HashedFSEntry{
			FSEntry: entry,
			Hash:    hash,
		})
	}

	// Compare dirs.
	dbDirs, err := s.db.AllDirs()
	if err != nil {
		return nil, err
	}
	for dirPath, dir := range fsDirs {
		if _, ok := dbDirs[dirPath]; !ok {
			changes.AddedDirs = append(changes.AddedDirs, dir)
		}
		delete(dbDirs, dirPath)
	}
	for _, dir := range dbDirs {
		changes.RemovedDirs = append(changes.RemovedDirs, dir)
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
		var newAdded []HashedFSEntry
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

// collectMetadata reads metadata from files for added and changed entries.
func (s *Scanner) collectMetadata(changes *ChangeSet) (*ScanResult, error) {
	result := &ScanResult{
		RemovedTracks: changes.Removed,
		Moves:         changes.Moves,
		AddedDirs:     changes.AddedDirs,
		RemovedDirs:   changes.RemovedDirs,
	}

	// Collect metadata for added tracks.
	for _, entry := range changes.Added {
		scanned, err := s.scanFile(entry.FSEntry, entry.Hash)
		if err != nil {
			slog.Warn("failed to scan added file", "dir", entry.Dir, "name", entry.Name, "error", err)
			continue
		}
		result.AddedTracks = append(result.AddedTracks, *scanned)
	}

	// Collect metadata for changed tracks (also recompute hash).
	for _, entry := range changes.Changed {
		hash, err := computePartialHash(s.fsPath(entry.Dir, entry.Name))
		if err != nil {
			slog.Warn("failed to hash changed file", "dir", entry.Dir, "name", entry.Name, "error", err)
			continue
		}
		scanned, err := s.scanFile(entry, hash)
		if err != nil {
			slog.Warn("failed to scan changed file", "dir", entry.Dir, "name", entry.Name, "error", err)
			continue
		}
		result.ChangedTracks = append(result.ChangedTracks, *scanned)
	}

	return result, nil
}

// scanFile opens an audio file and extracts metadata.
func (s *Scanner) scanFile(entry FSEntry, hash []byte) (*ScannedTrack, error) {
	fsPath := s.fsPath(entry.Dir, entry.Name)

	var src aurelib.Source
	var err error
	if fragment.IsFragment(fsPath) {
		src, err = fragment.New(fsPath)
	} else {
		src, err = aurelib.NewFileSource(fsPath)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", fsPath, err)
	}
	defer src.Destroy()

	tags := maputil.LowerCaseKeys(src.Tags())

	streamInfo := src.StreamInfo()

	metadata := TrackMetadata{
		Duration:     float64(src.Duration()) / float64(time.Second),
		BitRate:      src.BitRate(),
		SampleRate:   streamInfo.SampleRate,
		SampleFormat: streamInfo.SampleFormat(),
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

	// Collect attached image info.
	attachedImages := src.AttachedImages()
	imageInfos := make([]AttachedImageInfo, 0, len(attachedImages))
	for _, img := range attachedImages {
		imageInfos = append(imageInfos, AttachedImageInfo{
			MimeType: img.Format.MimeType(),
			Size:     len(img.Data),
		})
	}

	return &ScannedTrack{
		FSEntry:        entry,
		Hash:           hash,
		Tags:           tags,
		AttachedImages: imageInfos,
		Metadata:       metadata,
	}, nil
}

// marshalTrackJSON marshals the JSON fields of a ScannedTrack.
func marshalTrackJSON(t *ScannedTrack) (tagsJSON, imagesJSON, metadataJSON string, err error) {
	tagsBytes, err := json.Marshal(t.Tags)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to marshal tags: %w", err)
	}
	imagesBytes, err := json.Marshal(t.AttachedImages)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to marshal images: %w", err)
	}
	metadataBytes, err := json.Marshal(t.Metadata)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to marshal metadata: %w", err)
	}
	return string(tagsBytes), string(imagesBytes), string(metadataBytes), nil
}

// Apply writes a ScanResult to the database in a single transaction.
func (s *Scanner) Apply(result *ScanResult) error {
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
		stmt, err := tx.Prepare(`UPDATE tracks SET dir = ?, name = ?, mtime = ? WHERE dir = ? AND name = ?`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, m := range result.Moves {
			if _, err := stmt.Exec(m.NewDir, m.NewName, m.NewMtime, m.OldDir, m.OldName); err != nil {
				return fmt.Errorf("failed to apply move: %w", err)
			}
		}
	}

	// Changed.
	if len(result.ChangedTracks) > 0 {
		stmt, err := tx.Prepare(
			`UPDATE tracks SET mtime = ?, hash = ?, tags = ?, attached_images = ?, metadata = ?
			WHERE dir = ? AND name = ?`,
		)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for i := range result.ChangedTracks {
			t := &result.ChangedTracks[i]
			tagsJSON, imagesJSON, metadataJSON, err := marshalTrackJSON(t)
			if err != nil {
				return err
			}
			if _, err := stmt.Exec(t.Mtime, t.Hash, tagsJSON, imagesJSON, metadataJSON, t.Dir, t.Name); err != nil {
				return fmt.Errorf("failed to update track: %w", err)
			}
		}
	}

	// Added.
	if len(result.AddedTracks) > 0 {
		stmt, err := tx.Prepare(
			`INSERT INTO tracks (dir, name, mtime, hash, tags, attached_images, metadata)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
		)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for i := range result.AddedTracks {
			t := &result.AddedTracks[i]
			tagsJSON, imagesJSON, metadataJSON, err := marshalTrackJSON(t)
			if err != nil {
				return err
			}
			if _, err := stmt.Exec(t.Dir, t.Name, t.Mtime, t.Hash, tagsJSON, imagesJSON, metadataJSON); err != nil {
				return fmt.Errorf("failed to insert track: %w", err)
			}
		}
	}

	// Removed.
	if len(result.RemovedTracks) > 0 {
		stmt, err := tx.Prepare(`DELETE FROM tracks WHERE dir = ? AND name = ?`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, t := range result.RemovedTracks {
			if _, err := stmt.Exec(t.Dir, t.Name); err != nil {
				return fmt.Errorf("failed to delete track: %w", err)
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

	err = tx.Commit()
	if err == nil {
		committed = true
	}
	return err
}
