package mediadb

import (
	"bytes"
	"crypto/sha256"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/beakbeak/aurelius/pkg/aurelib"
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

// directoryImageRef is a reference to an image file in a track's directory.
// It stores only the path and MIME type; file contents are loaded on demand.
type directoryImageRef struct {
	path     string
	mimeType string
}

// coverImageRegexes defines image name priority tiers, from highest to lowest.
// Names matching earlier regexes sort before those matching later ones.
var coverImageRegexes = []*regexp.Regexp{
	regexp.MustCompile(`((?i)^front)|F$`),
	regexp.MustCompile(`(?i)^cover`),
	regexp.MustCompile(`(?i)front`),
	regexp.MustCompile(`(?i)cover`),
	regexp.MustCompile(`(?i)thumb`),
}

// coverImagePriority returns a sort priority for the given image name (without
// extension). Lower values sort first. Names not matching any pattern get the
// lowest priority.
func coverImagePriority(nameWithoutExt string) int {
	for i, re := range coverImageRegexes {
		if re.MatchString(nameWithoutExt) {
			return i
		}
	}
	return len(coverImageRegexes)
}

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

	// Sort by cover image priority tier, then lexicographically.
	slices.SortFunc(images, func(a, b imageEntry) int {
		aNameWithoutExt := strings.TrimSuffix(a.name, filepath.Ext(a.name))
		bNameWithoutExt := strings.TrimSuffix(b.name, filepath.Ext(b.name))

		aPriority := coverImagePriority(aNameWithoutExt)
		bPriority := coverImagePriority(bNameWithoutExt)

		if aPriority != bPriority {
			return aPriority - bPriority
		}
		return strings.Compare(a.name, b.name)
	})

	refs := make([]directoryImageRef, len(images))
	for i, img := range images {
		refs[i] = img.ref
	}
	return refs
}

// processedImage holds the result of processing a single image.
type processedImage struct {
	hash     [32]byte
	origHash [32]byte
	mimeType string
	data     []byte // nil if image already exists in DB
}

// imageHashCache provides thread-safe deduplication of image processing.
type imageHashCache struct {
	mu                      sync.Mutex
	originalHashToProcessed map[[32]byte][32]byte
}

// collectAndProcessTrackImages opens the audio file and scans the directory for
// images, processing each one. New images (not in cache) are sent through
// imageCh for the consumer to insert. The cache prevents duplicate processing
// both across scans (pre-loaded data) and within the current scan (worker
// updates). Returns the ordered image hashes for track_images linking.
func collectAndProcessTrackImages(
	trackFsPath string,
	cache *imageHashCache,
	imageCh chan<- processedImage,
) [][32]byte {
	var hashes [][32]byte
	seenHashes := make(map[[32]byte]bool)

	addImage := func(data []byte, mimeType, context string) {
		origHash := sha256.Sum256(data)

		// Check cache for already-processed image.
		cache.mu.Lock()
		if existingHash, ok := cache.originalHashToProcessed[origHash]; ok {
			cache.mu.Unlock()
			if !seenHashes[existingHash] {
				seenHashes[existingHash] = true
				hashes = append(hashes, existingHash)
			}
			return
		}
		cache.mu.Unlock()

		// Process the image (resize/re-encode if needed).
		processed, processedMime, hash, err := processImage(data, mimeType)
		if err != nil {
			slog.Warn("image processing failed", "context", "processImage", "path", context, "error", err)
			return
		}

		// Update cache.
		cache.mu.Lock()
		cache.originalHashToProcessed[origHash] = hash
		cache.mu.Unlock()

		if seenHashes[hash] {
			return
		}
		seenHashes[hash] = true

		imageCh <- processedImage{
			hash:     hash,
			origHash: origHash,
			mimeType: processedMime,
			data:     processed,
		}
		hashes = append(hashes, hash)
	}

	// Attached images from the audio file. The path is already resolved to
	// the real source file (even for fragments), so open directly.
	src, err := aurelib.NewFileSource(trackFsPath)
	if err != nil {
		slog.Warn("failed to open source for images", "path", trackFsPath, "error", err)
		return nil
	}
	for _, img := range src.AttachedImages() {
		addImage(img.Data, img.Format.MimeType(), trackFsPath)
	}
	src.Destroy()

	// Directory images.
	for _, ref := range collectDirectoryImagePaths(trackFsPath) {
		data, err := os.ReadFile(ref.path)
		if err != nil {
			slog.Warn("image processing failed", "context", "readFile", "path", ref.path, "error", err)
			continue
		}
		addImage(data, ref.mimeType, ref.path)
	}

	return hashes
}
