package mediadb

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

// testMediaPath returns the path to the test media directory.
func testMediaPath() string {
	return filepath.Join("..", "..", "test", "media")
}

func TestEventCoalescing(t *testing.T) {
	t.Parallel()

	// Create a temp dir with real files so Lstat works for Create events.
	tmpDir := t.TempDir()
	aPath := filepath.Join(tmpDir, "a.flac")
	bPath := filepath.Join(tmpDir, "b.flac")
	cPath := filepath.Join(tmpDir, "c.flac")
	hiddenPath := filepath.Join(tmpDir, ".hidden.flac")
	for _, p := range []string{aPath, bPath, cPath, hiddenPath} {
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatalf("failed to create test file: %v", err)
		}
	}

	w := &Watcher{rootPath: tmpDir, watchedDirs: make(map[string]bool)}

	tests := []struct {
		name     string
		events   []fsnotify.Event
		expected map[string]eventKind
	}{
		{
			name: "single create",
			events: []fsnotify.Event{
				{Name: aPath, Op: fsnotify.Create},
			},
			expected: map[string]eventKind{aPath: eventCreated},
		},
		{
			name: "single write",
			events: []fsnotify.Event{
				{Name: aPath, Op: fsnotify.Write},
			},
			expected: map[string]eventKind{aPath: eventModified},
		},
		{
			name: "single remove",
			events: []fsnotify.Event{
				{Name: aPath, Op: fsnotify.Remove},
			},
			expected: map[string]eventKind{aPath: eventRemoved},
		},
		{
			name: "rename treated as remove",
			events: []fsnotify.Event{
				{Name: aPath, Op: fsnotify.Rename},
			},
			expected: map[string]eventKind{aPath: eventRemoved},
		},
		{
			name: "create then write stays created",
			events: []fsnotify.Event{
				{Name: aPath, Op: fsnotify.Create},
				{Name: aPath, Op: fsnotify.Write},
				{Name: aPath, Op: fsnotify.Write},
			},
			expected: map[string]eventKind{aPath: eventCreated},
		},
		{
			name: "create then remove is no-op",
			events: []fsnotify.Event{
				{Name: aPath, Op: fsnotify.Create},
				{Name: aPath, Op: fsnotify.Remove},
			},
			expected: map[string]eventKind{},
		},
		{
			name: "modify then remove becomes removed",
			events: []fsnotify.Event{
				{Name: aPath, Op: fsnotify.Write},
				{Name: aPath, Op: fsnotify.Remove},
			},
			expected: map[string]eventKind{aPath: eventRemoved},
		},
		{
			name: "remove then create becomes modified",
			events: []fsnotify.Event{
				{Name: aPath, Op: fsnotify.Remove},
				{Name: aPath, Op: fsnotify.Create},
			},
			expected: map[string]eventKind{aPath: eventModified},
		},
		{
			name: "chmod only is ignored",
			events: []fsnotify.Event{
				{Name: aPath, Op: fsnotify.Chmod},
			},
			expected: map[string]eventKind{},
		},
		{
			name: "chmod with write is not ignored",
			events: []fsnotify.Event{
				{Name: aPath, Op: fsnotify.Chmod | fsnotify.Write},
			},
			expected: map[string]eventKind{aPath: eventModified},
		},
		{
			name: "hidden files are skipped",
			events: []fsnotify.Event{
				{Name: hiddenPath, Op: fsnotify.Create},
			},
			expected: map[string]eventKind{},
		},
		{
			name: "multiple independent files",
			events: []fsnotify.Event{
				{Name: aPath, Op: fsnotify.Create},
				{Name: bPath, Op: fsnotify.Remove},
				{Name: cPath, Op: fsnotify.Write},
			},
			expected: map[string]eventKind{
				aPath: eventCreated,
				bPath: eventRemoved,
				cPath: eventModified,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pending := make(map[string]*pendingEvent)
			for _, ev := range tt.events {
				w.handleEvent(ev, pending)
			}

			if len(pending) != len(tt.expected) {
				t.Fatalf("expected %d pending events, got %d: %v", len(tt.expected), len(pending), pendingKinds(pending))
			}

			for path, expectedKind := range tt.expected {
				got, ok := pending[path]
				if !ok {
					t.Errorf("expected pending event for %q, not found", path)
					continue
				}
				if got.kind != expectedKind {
					t.Errorf("for %q: expected kind %d, got %d", path, expectedKind, got.kind)
				}
			}
		})
	}
}

func pendingKinds(pending map[string]*pendingEvent) map[string]eventKind {
	result := make(map[string]eventKind, len(pending))
	for k, v := range pending {
		result[k] = v.kind
	}
	return result
}

// setupWatcherTest creates a temp directory, copies a test audio file into it,
// opens a DB, runs a full scan, and starts a watcher with a short quiet period.
// Returns the watcher, DB, scanner, temp dir path, and a channel that is
// signaled when a batch is applied.
func setupWatcherTest(t *testing.T) (w *Watcher, db *DB, scanner *Scanner, tmpDir string, batchApplied <-chan struct{}) {
	t.Helper()

	tmpDir = t.TempDir()
	mediaDir := testMediaPath()

	// Copy a test audio file into the temp dir.
	srcFile := filepath.Join(mediaDir, "test.ogg")
	srcData, err := os.ReadFile(srcFile)
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "test.ogg"), srcData, 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Open DB in temp dir.
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err = Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open DB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	scanner = NewScanner(db, tmpDir)
	if err := scanner.FullScan(); err != nil {
		t.Fatalf("full scan failed: %v", err)
	}

	applied := make(chan struct{}, 8)
	watcher, err := NewWatcher(scanner, tmpDir, WatcherConfig{
		QuietPeriod: 500 * time.Millisecond,
		OnBatchApplied: func() {
			applied <- struct{}{}
		},
	})
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	t.Cleanup(func() { watcher.Close() })

	return watcher, db, scanner, tmpDir, applied
}

// waitForBatch waits for a batch to be applied, or fails the test after a timeout.
func waitForBatch(t *testing.T, batchApplied <-chan struct{}) {
	t.Helper()
	select {
	case <-batchApplied:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for watcher batch")
	}
}

func TestWatcherAddFile(t *testing.T) {
	_, db, _, tmpDir, batchApplied := setupWatcherTest(t)

	// Copy another test file in.
	srcData, err := os.ReadFile(filepath.Join(testMediaPath(), "test.mp3"))
	if err != nil {
		t.Fatalf("failed to read source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "new.mp3"), srcData, 0o644); err != nil {
		t.Fatalf("failed to write new file: %v", err)
	}

	waitForBatch(t, batchApplied)

	track, err := db.GetTrack("new.mp3")
	if err != nil {
		t.Fatalf("GetTrack error: %v", err)
	}
	if track == nil {
		t.Fatal("expected new.mp3 to be in DB after watcher processed event")
	}
	if track.Metadata.Duration <= 0 {
		t.Errorf("expected positive duration, got %v", track.Metadata.Duration)
	}
}

func TestWatcherRemoveFile(t *testing.T) {
	_, db, _, tmpDir, batchApplied := setupWatcherTest(t)

	// Verify the file exists in DB.
	track, err := db.GetTrack("test.ogg")
	if err != nil || track == nil {
		t.Fatal("expected test.ogg to be in DB before removal")
	}

	// Remove it.
	if err := os.Remove(filepath.Join(tmpDir, "test.ogg")); err != nil {
		t.Fatalf("failed to remove file: %v", err)
	}

	waitForBatch(t, batchApplied)

	track, err = db.GetTrack("test.ogg")
	if err != nil {
		t.Fatalf("GetTrack error: %v", err)
	}
	if track != nil {
		t.Fatal("expected test.ogg to be removed from DB after watcher processed event")
	}
}

func TestWatcherModifyFile(t *testing.T) {
	_, db, _, tmpDir, batchApplied := setupWatcherTest(t)

	// Get original mtime.
	original, err := db.GetTrack("test.ogg")
	if err != nil || original == nil {
		t.Fatal("expected test.ogg to be in DB")
	}

	// Overwrite the file with the same content.
	srcData, err := os.ReadFile(filepath.Join(tmpDir, "test.ogg"))
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "test.ogg"), srcData, 0o644); err != nil {
		t.Fatalf("failed to overwrite file: %v", err)
	}
	// Ensure mtime is at least 1 second later (Unix seconds granularity).
	futureTime := time.Unix(original.Mtime+2, 0)
	if err := os.Chtimes(filepath.Join(tmpDir, "test.ogg"), futureTime, futureTime); err != nil {
		t.Fatalf("failed to set mtime: %v", err)
	}

	waitForBatch(t, batchApplied)

	updated, err := db.GetTrack("test.ogg")
	if err != nil || updated == nil {
		t.Fatal("expected test.ogg to still be in DB after modification")
	}
	if updated.Mtime <= original.Mtime {
		t.Errorf("expected mtime to increase: original=%d, updated=%d", original.Mtime, updated.Mtime)
	}
}

func TestWatcherMoveFile(t *testing.T) {
	_, db, _, tmpDir, batchApplied := setupWatcherTest(t)

	// Verify source exists.
	original, err := db.GetTrack("test.ogg")
	if err != nil || original == nil {
		t.Fatal("expected test.ogg to be in DB")
	}

	// Rename the file.
	oldPath := filepath.Join(tmpDir, "test.ogg")
	newPath := filepath.Join(tmpDir, "moved.ogg")
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatalf("rename failed: %v", err)
	}

	waitForBatch(t, batchApplied)

	// Old path should be gone.
	track, err := db.GetTrack("test.ogg")
	if err != nil {
		t.Fatalf("GetTrack error: %v", err)
	}
	if track != nil {
		t.Fatal("expected test.ogg to be gone from DB after move")
	}

	// New path should exist.
	track, err = db.GetTrack("moved.ogg")
	if err != nil {
		t.Fatalf("GetTrack error: %v", err)
	}
	if track == nil {
		t.Fatal("expected moved.ogg to be in DB after move")
	}
}

func TestWatcherAddDirectory(t *testing.T) {
	_, db, _, tmpDir, batchApplied := setupWatcherTest(t)

	// Create a subdirectory with a file.
	subDir := filepath.Join(tmpDir, "newdir")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}

	srcData, err := os.ReadFile(filepath.Join(testMediaPath(), "test.flac"))
	if err != nil {
		t.Fatalf("failed to read source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "track.flac"), srcData, 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	waitForBatch(t, batchApplied)

	// Check that the directory was added.
	subdirs, err := db.GetSubdirs("")
	if err != nil {
		t.Fatalf("GetSubdirs error: %v", err)
	}
	found := false
	for _, d := range subdirs {
		if d.Path == "newdir" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected newdir to be in dirs table")
	}

	// Check that the track was added.
	track, err := db.GetTrack("newdir/track.flac")
	if err != nil {
		t.Fatalf("GetTrack error: %v", err)
	}
	if track == nil {
		t.Fatal("expected newdir/track.flac to be in DB")
	}
}

func TestWatcherRemoveDirectory(t *testing.T) {
	_, db, _, tmpDir, batchApplied := setupWatcherTest(t)

	// Create a subdirectory with a track, then wait for the watcher to pick it up.
	subDir := filepath.Join(tmpDir, "toremove")
	if err := os.Mkdir(subDir, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	srcData, err := os.ReadFile(filepath.Join(testMediaPath(), "test.wav"))
	if err != nil {
		t.Fatalf("failed to read source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "song.wav"), srcData, 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	waitForBatch(t, batchApplied)

	// Verify it's in the DB.
	track, err := db.GetTrack("toremove/song.wav")
	if err != nil || track == nil {
		t.Fatal("expected toremove/song.wav to be in DB before removal")
	}

	// Remove the entire directory.
	if err := os.RemoveAll(subDir); err != nil {
		t.Fatalf("failed to remove dir: %v", err)
	}

	waitForBatch(t, batchApplied)

	// Track should be gone.
	track, err = db.GetTrack("toremove/song.wav")
	if err != nil {
		t.Fatalf("GetTrack error: %v", err)
	}
	if track != nil {
		t.Fatal("expected toremove/song.wav to be removed from DB")
	}

	// Dir should be gone.
	subdirs, err := db.GetSubdirs("")
	if err != nil {
		t.Fatalf("GetSubdirs error: %v", err)
	}
	for _, d := range subdirs {
		if d.Path == "toremove" {
			t.Fatal("expected toremove to be removed from dirs table")
		}
	}
}

func TestWatcherIgnoresNonTrackFiles(t *testing.T) {
	_, db, _, tmpDir, batchApplied := setupWatcherTest(t)

	// Create a non-track file and a track file at the same time.
	if err := os.WriteFile(filepath.Join(tmpDir, "notes.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("failed to write txt file: %v", err)
	}
	srcData, err := os.ReadFile(filepath.Join(testMediaPath(), "test.mp3"))
	if err != nil {
		t.Fatalf("failed to read source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "track.mp3"), srcData, 0o644); err != nil {
		t.Fatalf("failed to write mp3 file: %v", err)
	}

	waitForBatch(t, batchApplied)

	// Track file should be indexed.
	track, err := db.GetTrack("track.mp3")
	if err != nil {
		t.Fatalf("GetTrack error: %v", err)
	}
	if track == nil {
		t.Fatal("expected track.mp3 to be in DB")
	}

	// Non-track file should not be indexed.
	track, err = db.GetTrack("notes.txt")
	if err != nil {
		t.Fatalf("GetTrack error: %v", err)
	}
	if track != nil {
		t.Fatal("expected notes.txt to NOT be in DB")
	}
}

func TestWatcherReviveFile(t *testing.T) {
	_, db, _, tmpDir, batchApplied := setupWatcherTest(t)

	// Get the original track ID.
	original, err := db.GetTrack("test.ogg")
	if err != nil || original == nil {
		t.Fatal("expected test.ogg in DB")
	}
	originalID := original.ID

	// Remove the file and rescan to soft-delete it (bypass watcher to
	// ensure the removal is fully applied before we test revival).
	if err := os.Remove(filepath.Join(tmpDir, "test.ogg")); err != nil {
		t.Fatalf("failed to remove file: %v", err)
	}
	waitForBatch(t, batchApplied)

	track, err := db.GetTrack("test.ogg")
	if err != nil {
		t.Fatalf("GetTrack error: %v", err)
	}
	if track != nil {
		t.Fatal("expected test.ogg to be soft-deleted")
	}

	// Copy the same file back with a different name — should be revived.
	srcData, err := os.ReadFile(filepath.Join(testMediaPath(), "test.ogg"))
	if err != nil {
		t.Fatalf("failed to read source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "revived.ogg"), srcData, 0o644); err != nil {
		t.Fatalf("failed to write revived file: %v", err)
	}

	waitForBatch(t, batchApplied)

	// Old path should still be gone.
	track, err = db.GetTrack("test.ogg")
	if err != nil {
		t.Fatalf("GetTrack error: %v", err)
	}
	if track != nil {
		t.Fatal("expected test.ogg to remain gone")
	}

	// New path should exist with the original track ID preserved.
	track, err = db.GetTrack("revived.ogg")
	if err != nil {
		t.Fatalf("GetTrack error: %v", err)
	}
	if track == nil {
		t.Fatal("expected revived.ogg to be in DB after watcher revival")
	}
	if track.ID != originalID {
		t.Errorf("expected revived track to preserve original ID %d, got %d", originalID, track.ID)
	}
}

func TestWatcherBatchCallback(t *testing.T) {
	_, _, _, tmpDir, batchApplied := setupWatcherTest(t)

	var mu sync.Mutex
	callCount := 0

	// The batchApplied channel is driven by OnBatchApplied.
	// We'll count via the channel.
	go func() {
		for range batchApplied {
			mu.Lock()
			callCount++
			mu.Unlock()
		}
	}()

	// Create a file to trigger a batch.
	srcData, err := os.ReadFile(filepath.Join(testMediaPath(), "test.ogg"))
	if err != nil {
		t.Fatalf("failed to read source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "callback-test.ogg"), srcData, 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Wait for processing.
	time.Sleep(1500 * time.Millisecond)

	mu.Lock()
	if callCount == 0 {
		t.Error("expected OnBatchApplied to be called at least once")
	}
	mu.Unlock()
}
