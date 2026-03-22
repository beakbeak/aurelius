package mediadb

import "testing"

func TestGetFileType(t *testing.T) {
	tests := []struct {
		filename string
		want     FileType
	}{
		{"aurelius.yaml", FileTypeDirConfig},
		{"test.flac", FileTypeTrack},
		{"test.mp3", FileTypeTrack},
		{"test.ogg", FileTypeTrack},
		{"test.m3u", FileTypePlaylist},
		{"cover.jpg", FileTypeImage},
		{"cover.png", FileTypeImage},
		{"info.txt", FileTypeIgnored},
		{"info.nfo", FileTypeIgnored},
		{"unknown.xyz", FileTypeIgnored},
	}
	for _, tt := range tests {
		if got := GetFileType(tt.filename); got != tt.want {
			t.Errorf("GetFileType(%q) = %v, want %v", tt.filename, got, tt.want)
		}
	}
}
