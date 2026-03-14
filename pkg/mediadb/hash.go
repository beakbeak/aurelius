package mediadb

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

const hashChunkSize = 64 * 1024 // 64KB

// computePartialHash computes a partial hash of a file using the first and last
// 64KB of the file, combined with the file size.
func computePartialHash(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	h := sha256.New()

	// Write file size into hash.
	if err := binary.Write(h, binary.LittleEndian, info.Size()); err != nil {
		return nil, fmt.Errorf("failed to write size to hash: %w", err)
	}

	// Read first chunk.
	buf := make([]byte, hashChunkSize)
	n, _ := io.ReadFull(f, buf)
	h.Write(buf[:n])

	// Read last chunk (if file is large enough that it doesn't overlap).
	if info.Size() > int64(hashChunkSize)*2 {
		if _, err := f.Seek(-int64(hashChunkSize), io.SeekEnd); err != nil {
			return nil, fmt.Errorf("failed to seek: %w", err)
		}
		n, _ = io.ReadFull(f, buf)
		h.Write(buf[:n])
	}

	return h.Sum(nil), nil
}
