package mediadb

import (
	"os"
	"path/filepath"
	"testing"
)

// setupScannerTest creates a temp dir with a test audio file, opens a DB,
// and runs an initial full scan. Returns the scanner, DB, and temp dir path.
func setupScannerTest(t *testing.T) (*Scanner, *DB, string) {
	t.Helper()

	tmpDir := t.TempDir()

	srcData, err := os.ReadFile(filepath.Join(testMediaPath(), "test.ogg"))
	if err != nil {
		t.Fatalf("failed to read test file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "test.ogg"), srcData, 0o644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("failed to open DB: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	scanner := NewScanner(db, tmpDir)
	if err := scanner.FullScan(); err != nil {
		t.Fatalf("full scan failed: %v", err)
	}

	return scanner, db, tmpDir
}

func TestDetectRevivals(t *testing.T) {
	scanner, db, tmpDir := setupScannerTest(t)

	// Get the original track and its ID.
	original, err := db.GetTrack("test.ogg")
	if err != nil || original == nil {
		t.Fatal("expected test.ogg in DB")
	}
	originalID := original.ID

	// Remove the file and rescan to soft-delete it.
	if err := os.Remove(filepath.Join(tmpDir, "test.ogg")); err != nil {
		t.Fatalf("failed to remove file: %v", err)
	}
	if err := scanner.FullScan(); err != nil {
		t.Fatalf("second scan failed: %v", err)
	}

	// Verify the track is gone from the view.
	track, err := db.GetTrack("test.ogg")
	if err != nil {
		t.Fatalf("GetTrack error: %v", err)
	}
	if track != nil {
		t.Fatal("expected test.ogg to be soft-deleted")
	}

	// Copy the same file back with a different name.
	srcData, err := os.ReadFile(filepath.Join(testMediaPath(), "test.ogg"))
	if err != nil {
		t.Fatalf("failed to read source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "revived.ogg"), srcData, 0o644); err != nil {
		t.Fatalf("failed to write revived file: %v", err)
	}

	// Rescan — should detect the revival.
	if err := scanner.FullScan(); err != nil {
		t.Fatalf("third scan failed: %v", err)
	}

	// The old path should still be gone.
	track, err = db.GetTrack("test.ogg")
	if err != nil {
		t.Fatalf("GetTrack error: %v", err)
	}
	if track != nil {
		t.Fatal("expected test.ogg to remain gone")
	}

	// The new path should exist with the original track ID.
	track, err = db.GetTrack("revived.ogg")
	if err != nil {
		t.Fatalf("GetTrack error: %v", err)
	}
	if track == nil {
		t.Fatal("expected revived.ogg to be in DB")
	}
	if track.ID != originalID {
		t.Errorf("expected revived track to preserve original ID %d, got %d", originalID, track.ID)
	}
}

func TestDetectRevivalsAmbiguousHash(t *testing.T) {
	scanner, db, tmpDir := setupScannerTest(t)

	// Add a second copy of the same file under a different name.
	srcData, err := os.ReadFile(filepath.Join(testMediaPath(), "test.ogg"))
	if err != nil {
		t.Fatalf("failed to read source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "copy.ogg"), srcData, 0o644); err != nil {
		t.Fatalf("failed to write copy: %v", err)
	}
	if err := scanner.FullScan(); err != nil {
		t.Fatalf("second scan failed: %v", err)
	}

	// Remove both files and rescan to soft-delete them.
	os.Remove(filepath.Join(tmpDir, "test.ogg"))
	os.Remove(filepath.Join(tmpDir, "copy.ogg"))
	if err := scanner.FullScan(); err != nil {
		t.Fatalf("third scan failed: %v", err)
	}

	// Add the file back — should NOT be revived because two soft-deleted
	// tracks share the same hash (ambiguous).
	if err := os.WriteFile(filepath.Join(tmpDir, "new.ogg"), srcData, 0o644); err != nil {
		t.Fatalf("failed to write new file: %v", err)
	}
	if err := scanner.FullScan(); err != nil {
		t.Fatalf("fourth scan failed: %v", err)
	}

	track, err := db.GetTrack("new.ogg")
	if err != nil {
		t.Fatalf("GetTrack error: %v", err)
	}
	if track == nil {
		t.Fatal("expected new.ogg to be in DB")
	}
}

func TestDetectRevivalsSameLocation(t *testing.T) {
	scanner, db, tmpDir := setupScannerTest(t)

	original, err := db.GetTrack("test.ogg")
	if err != nil || original == nil {
		t.Fatal("expected test.ogg in DB")
	}
	originalID := original.ID

	// Remove file and rescan.
	if err := os.Remove(filepath.Join(tmpDir, "test.ogg")); err != nil {
		t.Fatalf("failed to remove file: %v", err)
	}
	if err := scanner.FullScan(); err != nil {
		t.Fatalf("second scan failed: %v", err)
	}

	// Put the same file back at the same path.
	srcData, err := os.ReadFile(filepath.Join(testMediaPath(), "test.ogg"))
	if err != nil {
		t.Fatalf("failed to read source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "test.ogg"), srcData, 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := scanner.FullScan(); err != nil {
		t.Fatalf("third scan failed: %v", err)
	}

	track, err := db.GetTrack("test.ogg")
	if err != nil {
		t.Fatalf("GetTrack error: %v", err)
	}
	if track == nil {
		t.Fatal("expected test.ogg to be in DB")
	}
	if track.ID != originalID {
		t.Errorf("expected track to preserve original ID %d, got %d", originalID, track.ID)
	}
}
