package mediadb

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

const defaultQuietPeriod = 2 * time.Second

// WatcherConfig holds configuration for the filesystem watcher.
type WatcherConfig struct {
	// QuietPeriod is the duration of inactivity after the last event
	// before the batch is processed. Default: defaultQuietPeriod.
	QuietPeriod time.Duration

	// OnBatchApplied is called after a batch of changes is successfully
	// applied to the database. Called in the watcher goroutine.
	OnBatchApplied func()
}

// Watcher monitors the filesystem for changes and updates the database.
type Watcher struct {
	scanner     *Scanner
	rootPath    string
	config      WatcherConfig
	fsWatcher   *fsnotify.Watcher
	watchedDirs map[string]bool // tracks which absolute paths are watched directories
	cancel      context.CancelFunc
	done        chan struct{}
}

type eventKind int

const (
	eventCreated eventKind = iota
	eventModified
	eventRemoved
)

type pendingEvent struct {
	kind  eventKind
	isDir bool
}

// NewWatcher creates and starts a filesystem watcher. It adds watches on all
// known subdirectories and starts a goroutine to process events.
func NewWatcher(scanner *Scanner, rootPath string, config WatcherConfig) (*Watcher, error) {
	if config.QuietPeriod <= 0 {
		config.QuietPeriod = defaultQuietPeriod
	}

	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	// Watch the root directory.
	if err := fsWatcher.Add(rootPath); err != nil {
		fsWatcher.Close()
		return nil, err
	}

	watchedDirs := map[string]bool{rootPath: true}

	// Watch all known subdirectories from the database.
	dirs, err := scanner.db.AllDirs()
	if err != nil {
		fsWatcher.Close()
		return nil, err
	}
	for _, d := range dirs {
		dirPath := filepath.Join(rootPath, filepath.FromSlash(d.Path))
		if err := fsWatcher.Add(dirPath); err != nil {
			slog.Warn("failed to watch directory", "path", dirPath, "error", err)
		}
		watchedDirs[dirPath] = true
	}

	ctx, cancel := context.WithCancel(context.Background())
	w := &Watcher{
		scanner:     scanner,
		rootPath:    rootPath,
		config:      config,
		fsWatcher:   fsWatcher,
		watchedDirs: watchedDirs,
		cancel:      cancel,
		done:        make(chan struct{}),
	}

	go w.run(ctx)

	slog.Info("filesystem watcher started", "root", rootPath, "dirs", len(dirs)+1)
	return w, nil
}

// Close stops the watcher goroutine and releases resources.
func (w *Watcher) Close() error {
	w.cancel()
	<-w.done
	return w.fsWatcher.Close()
}

// run is the main event loop for the watcher goroutine.
func (w *Watcher) run(ctx context.Context) {
	defer close(w.done)

	pending := make(map[string]*pendingEvent)
	timer := time.NewTimer(0)
	if !timer.Stop() {
		<-timer.C
	}
	timerActive := false

	for {
		select {
		case <-ctx.Done():
			if !timer.Stop() && timerActive {
				<-timer.C
			}
			return

		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event, pending)
			if !timerActive {
				timer.Reset(w.config.QuietPeriod)
				timerActive = true
			} else {
				if !timer.Stop() {
					<-timer.C
				}
				timer.Reset(w.config.QuietPeriod)
			}

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			slog.Warn("fsnotify error", "error", err)

		case <-timer.C:
			timerActive = false
			if len(pending) > 0 {
				w.processBatch(pending)
				pending = make(map[string]*pendingEvent)
			}
		}
	}
}

// handleEvent coalesces a single fsnotify event into the pending batch.
func (w *Watcher) handleEvent(event fsnotify.Event, pending map[string]*pendingEvent) {
	// Ignore pure chmod events.
	if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) == 0 {
		return
	}

	absPath := event.Name

	// Skip hidden files/dirs.
	if strings.HasPrefix(filepath.Base(absPath), ".") {
		return
	}

	// Determine the new event kind.
	var newKind eventKind
	switch {
	case event.Has(fsnotify.Create):
		newKind = eventCreated
	case event.Has(fsnotify.Write):
		newKind = eventModified
	case event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename):
		newKind = eventRemoved
	default:
		return
	}

	// For Create events, check if this is a directory and handle it.
	// If Lstat fails, assume it's a regular file — the file may have
	// already been moved away. Keeping it in pending ensures correct
	// coalescing with a subsequent Remove for the same path.
	if newKind == eventCreated {
		info, err := os.Lstat(absPath)
		if err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return
			}
			if info.IsDir() {
				w.handleNewDir(absPath, pending)
				return
			}
		}
	}

	// For Remove/Rename events, check if the path was a watched directory.
	if newKind == eventRemoved && w.watchedDirs[absPath] {
		delete(w.watchedDirs, absPath)
		pending[absPath] = &pendingEvent{kind: eventRemoved, isDir: true}
		return
	}

	// Apply coalescing rules.
	existing, exists := pending[absPath]
	if !exists {
		pending[absPath] = &pendingEvent{kind: newKind, isDir: false}
		return
	}

	switch existing.kind {
	case eventCreated:
		switch newKind {
		case eventCreated:
			// Duplicate create — still created.
		case eventModified:
			// Created then written — still created.
		case eventRemoved:
			// Created then removed — no-op.
			delete(pending, absPath)
		}
	case eventModified:
		switch newKind {
		case eventCreated:
			// Modified then created — still modified.
		case eventModified:
			// Still modified.
		case eventRemoved:
			existing.kind = eventRemoved
		}
	case eventRemoved:
		switch newKind {
		case eventCreated:
			// Removed then re-created — effectively modified.
			existing.kind = eventModified
		case eventModified:
			// Removed then modified — treat as modified.
			existing.kind = eventModified
		case eventRemoved:
			// Duplicate remove.
		}
	}
}

// handleNewDir adds watches for a new directory and walks it to discover
// any files already present (handles directory moves into the watched tree).
func (w *Watcher) handleNewDir(absPath string, pending map[string]*pendingEvent) {
	// Record the directory itself as created.
	pending[absPath] = &pendingEvent{kind: eventCreated, isDir: true}

	// Add a watch for the new directory.
	if err := w.fsWatcher.Add(absPath); err != nil {
		slog.Warn("failed to watch new directory", "path", absPath, "error", err)
	}
	w.watchedDirs[absPath] = true

	// Walk the new directory to discover contents.
	err := filepath.WalkDir(absPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // skip entries we can't access
		}
		if p == absPath {
			return nil
		}

		// Skip hidden files/dirs and symlinks.
		if strings.HasPrefix(d.Name(), ".") || d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			pending[p] = &pendingEvent{kind: eventCreated, isDir: true}
			if err := w.fsWatcher.Add(p); err != nil {
				slog.Warn("failed to watch new subdirectory", "path", p, "error", err)
			}
			w.watchedDirs[p] = true
			return nil
		}

		if !d.Type().IsRegular() {
			return nil
		}

		pending[p] = &pendingEvent{kind: eventCreated, isDir: false}
		return nil
	})
	if err != nil {
		slog.Warn("failed to walk new directory", "path", absPath, "error", err)
	}
}

// toLibraryPath converts an absolute filesystem path to a library-relative
// (dir, name) pair. Returns false if the path is outside the root.
func (w *Watcher) toLibraryPath(absPath string) (dir, name string, ok bool) {
	relPath, err := filepath.Rel(w.rootPath, absPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return "", "", false
	}
	libraryPath := filepath.ToSlash(relPath)
	dir = CleanLibraryPath(path.Dir(libraryPath))
	name = path.Base(libraryPath)
	return dir, name, true
}

// toLibraryDirPath converts an absolute directory path to a library-relative
// directory path. Returns false if the path is outside the root.
func (w *Watcher) toLibraryDirPath(absPath string) (string, bool) {
	relPath, err := filepath.Rel(w.rootPath, absPath)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return "", false
	}
	return CleanLibraryPath(filepath.ToSlash(relPath)), true
}

// processBatch converts buffered events into a ChangeSet and applies it.
func (w *Watcher) processBatch(events map[string]*pendingEvent) {
	changes := &ChangeSet{}

	// Initialize a WalkResult for this batch so that processDirConfigEvent
	// can store fragment metadata for collectMetadata to use.
	wr := &WalkResult{
		Fragments: make(map[string]resolvedFragment),
	}
	processedDirConfigs := make(map[string]bool)
	modifiedTrackDirs := make(map[string]bool)

	for absPath, ev := range events {
		if ev.isDir {
			w.processDirEvent(absPath, ev, changes)
		} else {
			w.processFileEvent(absPath, ev, wr, changes)

			dir, name, ok := w.toLibraryPath(absPath)
			if ok {
				switch GetFileType(name) {
				case FileTypeDirConfig:
					processedDirConfigs[dir] = true
				case FileTypeTrack:
					if ev.kind == eventModified || ev.kind == eventCreated {
						modifiedTrackDirs[dir] = true
					}
				case FileTypePlaylist, FileTypeImage, FileTypeIgnored:
				}
			}
		}
	}

	// When a source audio file is modified, re-evaluate the dir config for
	// that directory so fragments derived from it are updated.
	for dir := range modifiedTrackDirs {
		if processedDirConfigs[dir] {
			continue
		}
		configPath := w.scanner.fsPath(dir, dirConfigName)
		if _, err := os.Lstat(configPath); err != nil {
			continue
		}
		w.processDirConfigEvent(configPath, dir, &pendingEvent{kind: eventModified}, wr, changes)
	}

	if len(changes.Added) == 0 && len(changes.Changed) == 0 &&
		len(changes.Removed) == 0 && len(changes.AddedDirs) == 0 &&
		len(changes.RemovedDirs) == 0 &&
		len(changes.AddedPlaylists) == 0 && len(changes.ChangedPlaylists) == 0 &&
		len(changes.RemovedPlaylists) == 0 {
		return
	}

	// Deduplicate Removed entries. A track may appear both from an
	// individual file Remove event and from removeDirRecursive when its
	// parent directory is also removed.
	if len(changes.Removed) > 1 {
		seen := make(map[string]bool, len(changes.Removed))
		unique := changes.Removed[:0]
		for _, t := range changes.Removed {
			key := JoinLibraryPath(t.Dir, t.Name)
			if !seen[key] {
				seen[key] = true
				unique = append(unique, t)
			}
		}
		changes.Removed = unique
	}

	// Detect moves among added/removed.
	detectMoves(changes)
	if err := w.scanner.detectRevivals(changes); err != nil {
		slog.Error("watcher revival detection failed", "error", err)
		return
	}

	slog.Info("watcher batch",
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

	// Collect metadata for added/changed files.
	result, err := w.scanner.collectMetadata(wr, changes)
	if err != nil {
		slog.Error("watcher metadata collection failed", "error", err)
		return
	}

	// Apply to database.
	if err := w.scanner.Apply(wr, result); err != nil {
		slog.Error("watcher apply failed", "error", err)
		return
	}

	if w.config.OnBatchApplied != nil {
		w.config.OnBatchApplied()
	}
}

// processFileEvent handles a single file event in the batch.
func (w *Watcher) processFileEvent(absPath string, ev *pendingEvent, wr *WalkResult, changes *ChangeSet) {
	dir, name, ok := w.toLibraryPath(absPath)
	if !ok {
		return
	}

	switch GetFileType(name) {
	case FileTypeTrack:
		w.processTrackEvent(absPath, dir, name, ev, changes)
	case FileTypePlaylist:
		w.processPlaylistEvent(absPath, dir, name, ev, changes)
	case FileTypeImage:
		w.processImageFileEvent(dir, ev, changes)
	case FileTypeDirConfig:
		w.processDirConfigEvent(absPath, dir, ev, wr, changes)
	case FileTypeIgnored:
	}
}

// processTrackEvent handles a single track file event.
func (w *Watcher) processTrackEvent(absPath, dir, name string, ev *pendingEvent, changes *ChangeSet) {
	switch ev.kind {
	case eventCreated:
		info, err := os.Lstat(absPath)
		if err != nil {
			slog.Warn("watcher: failed to stat created file", "path", absPath, "error", err)
			return
		}
		if !info.Mode().IsRegular() {
			return
		}
		hash, err := computePartialHash(absPath)
		if err != nil {
			slog.Warn("watcher: failed to hash created file", "path", absPath, "error", err)
			return
		}
		changes.Added = append(changes.Added, HashedFSEntry{
			FSEntry: FSEntry{Dir: dir, Name: name, Mtime: info.ModTime().Unix()},
			Hash:    hash,
		})

	case eventModified:
		info, err := os.Lstat(absPath)
		if err != nil {
			slog.Warn("watcher: failed to stat modified file", "path", absPath, "error", err)
			return
		}
		if !info.Mode().IsRegular() {
			return
		}
		changes.Changed = append(changes.Changed, FSEntry{
			Dir: dir, Name: name, Mtime: info.ModTime().Unix(),
		})

	case eventRemoved:
		track, err := w.scanner.db.GetTrack(JoinLibraryPath(dir, name))
		if err != nil {
			slog.Warn("watcher: failed to look up removed track", "dir", dir, "name", name, "error", err)
			return
		}
		if track == nil {
			return
		}
		changes.Removed = append(changes.Removed, *track)
	}
}

// processPlaylistEvent handles a single playlist file event.
func (w *Watcher) processPlaylistEvent(absPath, dir, name string, ev *pendingEvent, changes *ChangeSet) {
	switch ev.kind {
	case eventCreated:
		info, err := os.Lstat(absPath)
		if err != nil {
			slog.Warn("watcher: failed to stat created playlist", "path", absPath, "error", err)
			return
		}
		if !info.Mode().IsRegular() {
			return
		}
		changes.AddedPlaylists = append(changes.AddedPlaylists, FSEntry{
			Dir: dir, Name: name, Mtime: info.ModTime().Unix(),
		})

	case eventModified:
		info, err := os.Lstat(absPath)
		if err != nil {
			slog.Warn("watcher: failed to stat modified playlist", "path", absPath, "error", err)
			return
		}
		if !info.Mode().IsRegular() {
			return
		}
		changes.ChangedPlaylists = append(changes.ChangedPlaylists, FSEntry{
			Dir: dir, Name: name, Mtime: info.ModTime().Unix(),
		})

	case eventRemoved:
		playlist, err := w.scanner.db.GetM3UPlaylist(JoinLibraryPath(dir, name))
		if err != nil {
			slog.Warn("watcher: failed to look up removed playlist", "dir", dir, "name", name, "error", err)
			return
		}
		if playlist == nil {
			return
		}
		changes.RemovedPlaylists = append(changes.RemovedPlaylists, *playlist)
	}
}

// processImageFileEvent handles a directory image file change by marking all
// tracks in the same directory as changed so their images are re-collected.
func (w *Watcher) processImageFileEvent(dir string, ev *pendingEvent, changes *ChangeSet) {
	tracks, err := w.scanner.db.GetTracksInDir(dir)
	if err != nil {
		slog.Warn("watcher: failed to look up tracks for image event", "dir", dir, "error", err)
		return
	}
	for _, t := range tracks {
		info, err := os.Lstat(w.scanner.fsPath(t.Dir, t.Name))
		if err != nil {
			continue
		}
		changes.Changed = append(changes.Changed, FSEntry{
			Dir: t.Dir, Name: t.Name, Mtime: info.ModTime().Unix(),
		})
	}
}

// processDirEvent handles a single directory event in the batch.
func (w *Watcher) processDirEvent(absPath string, ev *pendingEvent, changes *ChangeSet) {
	libraryDir, ok := w.toLibraryDirPath(absPath)
	if !ok {
		return
	}

	switch ev.kind {
	case eventCreated:
		if libraryDir == "" {
			return
		}
		changes.AddedDirs = append(changes.AddedDirs, Dir{
			Path:   libraryDir,
			Parent: CleanLibraryPath(path.Dir(libraryDir)),
		})

	case eventModified:
		// Directory modification (e.g. permissions change) — nothing to do.

	case eventRemoved:
		if libraryDir == "" {
			return
		}
		// Remove the directory, its tracks, and all subdirectories
		// recursively. Individual file Remove events may not fire for
		// atomic directory deletion.
		w.removeDirRecursive(libraryDir, changes)
	}
}

// removeDirRecursive adds a directory, its tracks, its playlists, and all
// subdirectories to the change set as removed.
func (w *Watcher) removeDirRecursive(libraryDir string, changes *ChangeSet) {
	changes.RemovedDirs = append(changes.RemovedDirs, Dir{
		Path:   libraryDir,
		Parent: CleanLibraryPath(path.Dir(libraryDir)),
	})

	tracks, err := w.scanner.db.GetTracksInDir(libraryDir)
	if err != nil {
		slog.Warn("watcher: failed to look up tracks in removed dir", "dir", libraryDir, "error", err)
	} else {
		for _, t := range tracks {
			changes.Removed = append(changes.Removed, t)
		}
	}

	playlists, err := w.scanner.db.GetM3UPlaylistsInDirIncludingEmpty(libraryDir)
	if err != nil {
		slog.Warn("watcher: failed to look up playlists in removed dir", "dir", libraryDir, "error", err)
	} else {
		changes.RemovedPlaylists = append(changes.RemovedPlaylists, playlists...)
	}

	subdirs, err := w.scanner.db.GetSubdirs(libraryDir)
	if err != nil {
		slog.Warn("watcher: failed to look up subdirs", "dir", libraryDir, "error", err)
		return
	}
	for _, d := range subdirs {
		w.removeDirRecursive(d.Path, changes)
	}
}

// processDirConfigEvent handles a change to an aurelius.yaml file. It diffs
// the new fragment definitions against the existing fragment tracks in the
// database and emits the appropriate add/change/remove entries.
func (w *Watcher) processDirConfigEvent(absPath, dir string, ev *pendingEvent, wr *WalkResult, changes *ChangeSet) {
	// Get existing fragment tracks from the database.
	existingTracks, err := w.scanner.db.GetTracksInDir(dir)
	if err != nil {
		slog.Warn("watcher: failed to look up tracks for dir config event", "dir", dir, "error", err)
		return
	}

	// Separate existing fragments from regular tracks.
	existingFragments := make(map[string]*Track)
	for i := range existingTracks {
		t := &existingTracks[i]
		if t.Metadata.Fragment != nil {
			existingFragments[t.Name] = t
		}
	}

	if ev.kind == eventRemoved {
		// Config removed: all fragments should be removed.
		for _, t := range existingFragments {
			changes.Removed = append(changes.Removed, *t)
		}
		return
	}

	// Config created or modified: parse and diff.
	config, err := LoadDirConfig(absPath)
	if err != nil {
		slog.Warn("watcher: failed to parse dir config", "path", absPath, "error", err)
		return
	}

	info, err := os.Lstat(absPath)
	if err != nil {
		slog.Warn("watcher: failed to stat dir config", "path", absPath, "error", err)
		return
	}
	configMtime := info.ModTime().Unix()

	// Build a set of non-fragment audio filenames from existing DB tracks.
	audioNameSet := make(map[string]bool)
	for _, t := range existingTracks {
		if t.Metadata.Fragment == nil {
			audioNameSet[t.Name] = true
		}
	}

	// Pre-compute config file hash.
	configHash, err := computeFullHash(absPath)
	if err != nil {
		slog.Warn("watcher: failed to hash dir config", "path", absPath, "error", err)
		return
	}

	// Expand new fragments.
	newFragments := make(map[string]bool)
	sourceFragmentCount := make(map[string]int)

	for i := range config.Fragments {
		def := &config.Fragments[i]
		sourceFile := def.Source
		if !audioNameSet[sourceFile] {
			slog.Warn("watcher: fragment source file not found",
				"dir", dir, "source", sourceFile)
			continue
		}

		sourceFragmentCount[sourceFile]++
		fragIdx := sourceFragmentCount[sourceFile]
		syntheticName := MakeFragmentName(sourceFile, fragIdx)
		newFragments[syntheticName] = true

		// Compute mtime as max(configMtime, sourceMtime).
		mtime := configMtime
		sourceFSPath := w.scanner.fsPath(dir, sourceFile)
		if sourceInfo, err := os.Lstat(sourceFSPath); err == nil {
			if sourceMtime := sourceInfo.ModTime().Unix(); sourceMtime > mtime {
				mtime = sourceMtime
			}
		}

		if existing, ok := existingFragments[syntheticName]; ok {
			// Fragment exists — check if changed.
			if existing.Mtime != mtime {
				changes.Changed = append(changes.Changed, FSEntry{
					Dir: dir, Name: syntheticName, Mtime: mtime,
				})
			}
		} else {
			// New fragment.
			sourceHash, err := computePartialHash(sourceFSPath)
			if err != nil {
				slog.Warn("watcher: failed to hash fragment source", "dir", dir, "name", sourceFile, "error", err)
				continue
			}
			fragHash := computeFragmentHash(configHash, sourceHash, fragIdx)
			changes.Added = append(changes.Added, HashedFSEntry{
				FSEntry: FSEntry{Dir: dir, Name: syntheticName, Mtime: mtime},
				Hash:    fragHash,
			})
		}

		// Store resolved fragment for metadata collection.
		libraryPath := JoinLibraryPath(dir, syntheticName)
		sourceHash, _ := computePartialHash(w.scanner.fsPath(dir, sourceFile))
		wr.Fragments[libraryPath] = resolvedFragment{
			SourceFSPath: w.scanner.fsPath(dir, sourceFile),
			SourceFile:   sourceFile,
			Config:       def,
			Index:        fragIdx,
			hash:         computeFragmentHash(configHash, sourceHash, fragIdx),
		}
	}

	// Remove fragments that no longer exist in the config.
	for name, t := range existingFragments {
		if !newFragments[name] {
			changes.Removed = append(changes.Removed, *t)
		}
	}
}
