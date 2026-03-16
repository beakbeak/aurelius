package mediadb

import (
	"bytes"
	"crypto/sha256"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/image/draw"
)

const (
	maxImageDataSize = 100 * 1024
	maxImageDim      = 1024
)

// Ensure image format decoders are registered.
func init() {
	// Force registration of decoders via blank imports in the import block
	// above. The gif, jpeg, and png packages register themselves in their
	// init() functions. We reference them here to silence unused-import
	// errors while keeping the imports explicit.
	_ = gif.Decode
	_ = jpeg.Decode
	_ = png.Decode
}

// processImage returns the image data (possibly resized/re-encoded), its MIME
// type, and its SHA-256 hash. Images <= 100 KiB are returned as-is. Larger
// images are decoded, resized to fit 1024x1024, and re-encoded as JPEG with
// iteratively decreasing quality until the result is <= 100 KiB.
func processImage(data []byte, mimeType string) ([]byte, string, [32]byte, error) {
	if len(data) <= maxImageDataSize {
		hash := sha256.Sum256(data)
		return data, mimeType, hash, nil
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, "", [32]byte{}, err
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()

	if w > maxImageDim || h > maxImageDim {
		ratio := float64(w) / float64(h)
		if w > h {
			w = maxImageDim
			h = int(float64(w) / ratio)
		} else {
			h = maxImageDim
			w = int(float64(h) * ratio)
		}
		if w < 1 {
			w = 1
		}
		if h < 1 {
			h = 1
		}
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		draw.BiLinear.Scale(dst, dst.Bounds(), img, bounds, draw.Src, nil)
		img = dst
	}

	var last []byte
	for quality := 85; quality >= 15; quality -= 10 {
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
			return nil, "", [32]byte{}, err
		}
		last = buf.Bytes()
		if len(last) <= maxImageDataSize {
			break
		}
	}
	hash := sha256.Sum256(last)
	return last, "image/jpeg", hash, nil
}

// processImageFile reads an image file from disk, processes it, and returns the
// result.
func processImageFile(path string, mimeType string) ([]byte, string, [32]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", [32]byte{}, err
	}
	return processImage(data, mimeType)
}

// directoryImageRef is a reference to an image file in a track's directory.
// It stores only the path and MIME type; file contents are loaded on demand.
type directoryImageRef struct {
	path     string
	mimeType string
}

var coverImageRegex = regexp.MustCompile(`(?i)front|cover|thumb|F$`)

var knownImageExts = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
}

// collectDirectoryImagePaths discovers image files in the same directory as the
// given track, returning sorted references without reading file contents. Cover
// images (matching common naming patterns) are sorted first.
func collectDirectoryImagePaths(trackFsPath string) []directoryImageRef {
	dir := filepath.Dir(trackFsPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	type imageEntry struct {
		ref  directoryImageRef
		name string
	}

	var images []imageEntry
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		mime, ok := knownImageExts[ext]
		if !ok {
			continue
		}
		images = append(images, imageEntry{
			ref: directoryImageRef{
				path:     filepath.Join(dir, entry.Name()),
				mimeType: mime,
			},
			name: entry.Name(),
		})
	}

	// Sort cover images first, then lexicographically.
	slices.SortFunc(images, func(a, b imageEntry) int {
		aNameWithoutExt := strings.TrimSuffix(a.name, filepath.Ext(a.name))
		bNameWithoutExt := strings.TrimSuffix(b.name, filepath.Ext(b.name))

		aCover := coverImageRegex.MatchString(aNameWithoutExt)
		bCover := coverImageRegex.MatchString(bNameWithoutExt)

		if aCover && !bCover {
			return -1
		}
		if !aCover && bCover {
			return 1
		}
		return strings.Compare(a.name, b.name)
	})

	refs := make([]directoryImageRef, len(images))
	for i, img := range images {
		refs[i] = img.ref
	}
	return refs
}
