package mediadb

import (
	"os"
	"testing"
	"time"
)

func TestMakeFragmentName(t *testing.T) {
	tests := []struct {
		source string
		index  int
		want   string
	}{
		{"test.flac", 1, "test.flac::1"},
		{"test.flac", 42, "test.flac::42"},
	}
	for _, tt := range tests {
		if got := MakeFragmentName(tt.source, tt.index); got != tt.want {
			t.Errorf("MakeFragmentName(%q, %d) = %q, want %q", tt.source, tt.index, got, tt.want)
		}
	}
}

func TestLoadDirConfig(t *testing.T) {
	t.Run("valid YAML", func(t *testing.T) {
		f, err := os.CreateTemp(t.TempDir(), "aurelius*.yaml")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.WriteString(`
fragments:
  - source: track01.flac
    start: 2s
    end: 5s
  - source: track02.mp3
    start: 1m30s
    artist: Test Artist
    title: Test Title
    album: Test Album
`); err != nil {
			t.Fatal(err)
		}
		f.Close()

		config, err := LoadDirConfig(f.Name())
		if err != nil {
			t.Fatalf("LoadDirConfig failed: %v", err)
		}

		if len(config.Fragments) != 2 {
			t.Fatalf("expected 2 fragments, got %d", len(config.Fragments))
		}

		frag0 := config.Fragments[0]
		if frag0.Source != "track01.flac" {
			t.Errorf("fragment 0: source = %q, want %q", frag0.Source, "track01.flac")
		}
		if frag0.Start != 2*time.Second {
			t.Errorf("fragment 0: start = %v, want %v", frag0.Start, 2*time.Second)
		}
		if frag0.End != 5*time.Second {
			t.Errorf("fragment 0: end = %v, want %v", frag0.End, 5*time.Second)
		}

		frag1 := config.Fragments[1]
		if frag1.Source != "track02.mp3" {
			t.Errorf("fragment 1: source = %q, want %q", frag1.Source, "track02.mp3")
		}
		if frag1.Artist != "Test Artist" {
			t.Errorf("fragment 1: artist = %q, want %q", frag1.Artist, "Test Artist")
		}
		if frag1.Title != "Test Title" {
			t.Errorf("fragment 1: title = %q, want %q", frag1.Title, "Test Title")
		}
		if frag1.Album != "Test Album" {
			t.Errorf("fragment 1: album = %q, want %q", frag1.Album, "Test Album")
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := LoadDirConfig("/nonexistent/aurelius.yaml")
		if err == nil {
			t.Error("expected error for missing file")
		}
	})

	t.Run("empty fragments", func(t *testing.T) {
		f, err := os.CreateTemp(t.TempDir(), "aurelius*.yaml")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.WriteString("fragments: []\n"); err != nil {
			t.Fatal(err)
		}
		f.Close()

		config, err := LoadDirConfig(f.Name())
		if err != nil {
			t.Fatalf("LoadDirConfig failed: %v", err)
		}
		if len(config.Fragments) != 0 {
			t.Errorf("expected 0 fragments, got %d", len(config.Fragments))
		}
	})
}
