package media_test

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/beakbeak/aurelius/internal/media"
)

var (
	// After confirming new test results are correct, set to true and re-run
	// tests to update baseline.json
	updateBaselines = os.Getenv("UPDATE_BASELINES") == "1"

	testDataRoot     = filepath.Join("..", "..", "test")
	testMediaPath    = filepath.Join(testDataRoot, "media")
	testStoragePath  = filepath.Join(testDataRoot, "storage")
	baselineJsonPath = filepath.Join(testDataRoot, "baseline.json")

	apiPrefix = "/prefix"

	testFiles = []string{
		"test.flac",
		"test.mp3",
		"test.ogg",
		"test.wav",
		"test.mka",
		"foo/another directory/test.m4a",
		"test.flac::001",
		"test.flac::002",
		"test.flac::003",
	}
)

// Baselines ///////////////////////////////////////////////////////////////////

type Baseline struct {
	Info         map[string]interface{} // result of info request
	StreamHashes map[string]string      // query string -> stream checksum
}

type BaselineMap map[string]Baseline // file name -> baseline

func readBaselines() BaselineMap {
	file, err := os.Open(baselineJsonPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(BaselineMap)
		}
		panic(fmt.Sprintf("Open(\"%s\") failed: %v", baselineJsonPath, err))
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	decoder := json.NewDecoder(reader)

	var out BaselineMap

	for decoder.More() {
		if err = decoder.Decode(&out); err != nil {
			panic(fmt.Sprintf("decoding \"%s\" failed: %v", baselineJsonPath, err))
		}
	}

	return out
}

func writeBaselines(b BaselineMap) {
	file, err := os.Create(baselineJsonPath)
	if err != nil {
		panic(fmt.Sprintf("Create(\"%s\") failed: %v", baselineJsonPath, err))
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")

	if err = encoder.Encode(b); err != nil {
		panic(fmt.Sprintf("encoding \"%s\" failed: %v", baselineJsonPath, err))
	}
}

// General utilities ///////////////////////////////////////////////////////////

func simpleRequestWithStatus(
	t *testing.T,
	ml *media.Library,
	method string,
	path string,
	requestBody string,
) ([]byte, int) {
	t.Helper()
	t.Log(method, path)
	w := httptest.NewRecorder()
	ml.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(requestBody)))
	response := w.Result()
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	return responseBody, response.StatusCode
}

func simpleRequest(
	t *testing.T,
	ml *media.Library,
	method string,
	path string,
	requestBody string,
) []byte {
	t.Helper()
	responseBody, statusCode := simpleRequestWithStatus(t, ml, method, path, requestBody)
	if statusCode != http.StatusOK {
		t.Fatalf("%s '%s' failed with code %v:\n%v", method, path, statusCode, string(responseBody))
	}
	return responseBody
}

func simpleRequestShouldFail(
	t *testing.T,
	ml *media.Library,
	method string,
	path string,
	requestBody string,
) {
	t.Helper()
	responseBody, statusCode := simpleRequestWithStatus(t, ml, method, path, requestBody)
	if statusCode == http.StatusOK {
		t.Fatalf("%s '%s' succeeded but should have failed:\n%v", method, path, responseBody)
	}
}

func writeStringToFile(
	t *testing.T,
	path string,
	contents string,
) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("os.Create(\"%s\") failed: %v", path, err)
	}
	defer file.Close()

	switch n, err := file.WriteString(contents); {
	case n != len(contents):
		t.Fatalf("expected WriteString() to return %v, got %v", len(contents), n)
	case err != nil:
		t.Fatalf("WriteString() failed: %v", err)
	}
}

type ArgTableEntry struct {
	Key    string
	Values []string
}

// combineQueryArgs generates all combinations of the key/value pairs contained
// in each element of argTables and returns them in query string format.
// Empty values are skipped.
func combineQueryArgs(argTables [][]ArgTableEntry) []string {
	var queryStrings []string

	for _, argTable := range argTables {
		tableIndices := make([]int, len(argTable))
		for {
			queryString := ""
			for i := 0; i < len(tableIndices); i++ {
				value := argTable[i].Values[tableIndices[i]]
				if value == "" {
					continue
				}

				if queryString == "" {
					queryString = "?"
				} else {
					queryString += "&"
				}

				queryString += argTable[i].Key + "=" + value
			}

			queryStrings = append(queryStrings, queryString)

			i := 0
			for ; i < len(tableIndices); i++ {
				tableIndices[i]++
				if tableIndices[i] < len(argTable[i].Values) {
					break
				}
				tableIndices[i] = 0
			}
			if i >= len(tableIndices) {
				break
			}
		}
	}

	return queryStrings
}

// JSON utilities //////////////////////////////////////////////////////////////

func jsonEqual(
	t *testing.T,
	s1 []byte,
	s2 []byte,
) bool {
	t.Helper()
	var buf1, buf2 bytes.Buffer
	var err error

	err = json.Compact(&buf1, s1)
	if err != nil {
		t.Fatalf("json.Compact() failed: %v", err)
	}

	err = json.Compact(&buf2, s2)
	if err != nil {
		t.Fatalf("json.Compact() failed: %v", err)
	}

	return reflect.DeepEqual(buf1.Bytes(), buf2.Bytes())
}

func indentJson(
	t *testing.T,
	s []byte,
) string {
	t.Helper()
	var buf bytes.Buffer
	err := json.Indent(&buf, s, "", "    ")
	if err != nil {
		t.Fatalf("json.Indent() failed: %v", err)
	}
	return buf.String()
}

func unmarshalJson(
	t *testing.T,
	data []byte,
	v interface{},
) {
	t.Helper()
	err := json.Unmarshal(data, v)
	if err != nil {
		t.Fatalf("json.Unmarshal(%q) failed: %v\n", string(data), err)
	}
}

func removeJsonElement(
	obj interface{},
	path ...string,
) bool {
	for index, pathElement := range path {
		stringMap, ok := obj.(map[string]interface{})
		if !ok {
			return false
		}

		if index == (len(path) - 1) {
			if _, ok = stringMap[pathElement]; ok {
				delete(stringMap, pathElement)
				return true
			}
			return false
		}

		obj = stringMap[pathElement]
	}
	return false
}

// Library utilities ///////////////////////////////////////////////////////////

func createDefaultLibrary(t *testing.T) *media.Library {
	t.Helper()
	clearStorage(t)

	mlConfig := media.NewLibraryConfig()
	mlConfig.RootPath = testMediaPath
	mlConfig.StoragePath = testStoragePath
	mlConfig.Prefix = apiPrefix
	mlConfig.ThrottleStreaming = false
	mlConfig.DeterministicStreaming = true
	ml, err := media.NewLibrary(mlConfig)
	if err != nil {
		t.Fatalf("failed to create Library: %v", err)
	}
	t.Cleanup(func() { ml.Close() })
	return ml
}

func api(elements ...string) string {
	return path.Join(apiPrefix, path.Join(elements...))
}
func atPath(elements ...string) string {
	return "at:" + url.PathEscape(path.Join(elements...))
}
func dirAt(path string, elements ...string) string {
	return api(append([]string{"dirs", atPath(path)}, elements...)...)
}
func trackAt(path string, elements ...string) string {
	return api(append([]string{"tracks", atPath(path)}, elements...)...)
}
func playlistAt(path string, elements ...string) string {
	return api(append([]string{"playlists", atPath(path)}, elements...)...)
}

func clearStorage(t *testing.T) {
	t.Helper()
	switch _, err := os.Stat(testStoragePath); {
	case os.IsNotExist(err):
		return
	case err != nil:
		t.Fatalf("\"%s\" exists but Stat() failed: %v", testStoragePath, err)
	}

	if err := os.RemoveAll(testStoragePath); err != nil {
		t.Fatalf("RemoveAll(\"%s\") failed: %v", testStoragePath, err)
	}
}

func isFavorite(
	t *testing.T,
	ml *media.Library,
	path string,
) bool {
	t.Helper()
	jsonBytes := simpleRequest(t, ml, "GET", trackAt(path), "")

	var track struct{ Favorite bool }
	unmarshalJson(t, jsonBytes, &track)

	return track.Favorite
}

func getPlaylistEntry(
	t *testing.T,
	ml *media.Library,
	path string,
	pos int,
) media.PlaylistTrack {
	t.Helper()
	url := fmt.Sprintf("%s/tracks/%v", path, pos)
	jsonBytes := simpleRequest(t, ml, "GET", url, "")

	var entry media.PlaylistTrack
	unmarshalJson(t, jsonBytes, &entry)
	return entry
}

func getPlaylistEntryWithPrefix(
	t *testing.T,
	ml *media.Library,
	basePath string,
	pos int,
	prefix string,
) media.PlaylistTrack {
	t.Helper()
	url := fmt.Sprintf("%s/tracks/%v?prefix=%s", basePath, pos, prefix)
	jsonBytes := simpleRequest(t, ml, "GET", url, "")

	var entry media.PlaylistTrack
	unmarshalJson(t, jsonBytes, &entry)
	return entry
}

func getPlaylistEntryShouldFail(
	t *testing.T,
	ml *media.Library,
	path string,
	pos int,
) {
	t.Helper()
	url := fmt.Sprintf("%s/tracks/%v", path, pos)
	jsonBytes := simpleRequest(t, ml, "GET", url, "")

	if !jsonEqual(t, jsonBytes, []byte("null")) {
		t.Fatalf("expected %s to be null, got %v", url, string(jsonBytes))
	}
}

func getPlaylistEntryShouldFailWithPrefix(
	t *testing.T,
	ml *media.Library,
	basePath string,
	pos int,
	prefix string,
) {
	t.Helper()
	url := fmt.Sprintf("%s/tracks/%v?prefix=%s", basePath, pos, prefix)
	jsonBytes := simpleRequest(t, ml, "GET", url, "")

	if !jsonEqual(t, jsonBytes, []byte("null")) {
		t.Fatalf("expected %s to be null, got %v", url, string(jsonBytes))
	}
}

func getPlaylistLength(
	t *testing.T,
	ml *media.Library,
	apiPath string,
) int {
	t.Helper()
	jsonBytes := simpleRequest(t, ml, "GET", apiPath, "")

	var info struct{ Length int }
	unmarshalJson(t, jsonBytes, &info)

	return info.Length
}

func getDirInfo(
	t *testing.T,
	ml *media.Library,
	path string,
) media.Dir {
	t.Helper()
	jsonBytes := simpleRequest(t, ml, "GET", path, "")

	var info media.Dir
	unmarshalJson(t, jsonBytes, &info)
	return info
}

// Tests ///////////////////////////////////////////////////////////////////////

func TestTrack(t *testing.T) {
	ml := createDefaultLibrary(t)

	simpleRequestShouldFail(t, ml, "GET", trackAt("nonexistent.mp3"), "")

	baselines := readBaselines()

	for _, rangePath := range testFiles {
		path := rangePath

		t.Run(path, func(t *testing.T) {
			if !updateBaselines {
				t.Parallel()
			}

			body := simpleRequest(t, ml, "GET", trackAt(path), "")

			var trackInfo map[string]interface{}
			if err := json.Unmarshal(body, &trackInfo); err != nil {
				t.Fatalf("failed to decode JSON: %v\n%s", err, indentJson(t, body))
			}

			// FFmpeg returns inconsistent values for "encoder" tag with .mka files
			removeJsonElement(trackInfo, "tags", "encoder")

			baseline := baselines[path]

			if updateBaselines {
				baseline.Info = trackInfo
				baselines[path] = baseline
				return
			}

			if !reflect.DeepEqual(trackInfo, baseline.Info) {
				t.Fatalf("unexpected JSON: %v", indentJson(t, body))
			}
		})
	}

	if updateBaselines {
		writeBaselines(baselines)
	}
}

func TestFavorite(t *testing.T) {
	ml := createDefaultLibrary(t)

	simpleRequestShouldFail(t, ml, "POST", trackAt("nonexistent.mp3", "favorite"), "")
	simpleRequestShouldFail(t, ml, "POST", trackAt("nonexistent.mp3", "unfavorite"), "")

	path := testFiles[1]

	checkFavoriteInDirListing := func(expectedFavorite bool) {
		t.Helper()
		dirInfo := getDirInfo(t, ml, dirAt(""))
		idx := slices.IndexFunc(dirInfo.Tracks, func(u media.Track) bool { return u.Name == path })
		if idx < 0 {
			t.Fatalf("track %q not found in directory listing", path)
		}
		if dirInfo.Tracks[idx].Favorite != expectedFavorite {
			t.Fatalf("expected favorite to be %t in directory listing for %q, got %t", expectedFavorite, path, dirInfo.Tracks[idx].Favorite)
		}
	}

	simpleRequest(t, ml, "POST", trackAt(path, "unfavorite"), "")

	if isFavorite(t, ml, path) {
		t.Fatalf("expected 'favorite' to be false for %q", path)
	}
	checkFavoriteInDirListing(false)

	simpleRequest(t, ml, "POST", trackAt(path, "favorite"), "")

	if !isFavorite(t, ml, path) {
		t.Fatalf("expected 'favorite' to be true for %q", path)
	}
	checkFavoriteInDirListing(true)

	simpleRequest(t, ml, "POST", trackAt(path, "unfavorite"), "")

	if isFavorite(t, ml, path) {
		t.Fatalf("expected 'favorite' to be false for %q", path)
	}
	checkFavoriteInDirListing(false)
}

func TestFavoritePaths(t *testing.T) {
	ml := createDefaultLibrary(t)

	// Empty favorites should return length 0.
	if length := getPlaylistLength(t, ml, api("playlists", "favorites")); length != 0 {
		t.Fatalf("expected empty favorites to have 0 entries, got %v", length)
	}

	for i := 0; i < 2; i++ {
		for _, path := range testFiles {
			simpleRequest(t, ml, "POST", trackAt(path, "favorite"), "")
		}
	}

	length := getPlaylistLength(t, ml, api("playlists", "favorites"))
	expectedLength := len(testFiles)
	if length != expectedLength {
		t.Fatalf("expected favorites to have %v entries, got %v", expectedLength, length)
	}

	// Fetch and compare the paths of favorites (order-independent).
	favoritesPath := api("playlists", "favorites")
	var actualPaths []string
	for i := 0; i < length; i++ {
		entry := getPlaylistEntry(t, ml, favoritesPath, i)
		actualPaths = append(actualPaths, entry.Path)
	}

	// Build expected paths.
	var expectedPaths []string
	for _, path := range testFiles {
		expectedPaths = append(expectedPaths, trackAt(path))
	}

	slices.Sort(actualPaths)
	slices.Sort(expectedPaths)

	if !slices.Equal(actualPaths, expectedPaths) {
		t.Fatalf("favorites paths mismatch:\nexpected: %v\ngot:      %v", expectedPaths, actualPaths)
	}

	// Test prefix filtering (directory-based).
	t.Run("PrefixFiltering", func(t *testing.T) {
		// Test with "foo" prefix - should match foo/another directory/test.m4a
		prefixLength := getPlaylistLength(t, ml, api("playlists", "favorites")+"?prefix=foo")
		expectedPrefixLength := 1
		if prefixLength != expectedPrefixLength {
			t.Errorf("expected prefix 'foo' to match %v entries, got %v", expectedPrefixLength, prefixLength)
		}

		// Test with "foo/" prefix - should also match
		prefixLength = getPlaylistLength(t, ml, api("playlists", "favorites")+"?prefix=foo/")
		if prefixLength != expectedPrefixLength {
			t.Errorf("expected prefix 'foo/' to match %v entries, got %v", expectedPrefixLength, prefixLength)
		}

		// Test with non-matching prefix
		prefixLength = getPlaylistLength(t, ml, api("playlists", "favorites")+"?prefix=nonexistent")
		expectedPrefixLength = 0
		if prefixLength != expectedPrefixLength {
			t.Errorf("expected prefix 'nonexistent' to match %v entries, got %v", expectedPrefixLength, prefixLength)
		}

		// Test fetching specific entries with prefix
		testPrefix := "foo"
		basePath := api("playlists", "favorites")
		prefixPath := basePath + "?prefix=" + testPrefix
		prefixLength = getPlaylistLength(t, ml, prefixPath)

		// Verify we can fetch each entry in the filtered list
		for i := 0; i < prefixLength; i++ {
			entry := getPlaylistEntryWithPrefix(t, ml, basePath, i, testPrefix)
			if entry.Path == "" {
				t.Errorf("expected non-empty path for prefix entry %d", i)
			}
			if entry.Pos != i {
				t.Errorf("expected position %d for prefix entry, got %d", i, entry.Pos)
			}
		}

		// Test that positions beyond the filtered length return null
		getPlaylistEntryShouldFailWithPrefix(t, ml, basePath, prefixLength, testPrefix)
	})
}

func TestM3UPlaylist(t *testing.T) {
	playlist := []string{
		"test.flac",
		"test.mp3",
		"test.ogg",
		"test.wav",
		"test.mp3",
		"test.ogg",
		"test.flac",
		"test.wav",
		"test.flac",
	}

	playlistName := "temp-playlist.m3u"
	playlistFilePath := filepath.Join(testMediaPath, playlistName)
	playlistLibraryPath := playlistAt(playlistName)

	writeStringToFile(t, playlistFilePath, strings.Join(playlist, "\n"))
	defer (func() {
		if err := os.Remove(playlistFilePath); err != nil {
			t.Logf("Remove(\"%s\") failed: %v", playlistFilePath, err)
			t.Fail()
		}
	})()

	ml := createDefaultLibrary(t)

	if length := getPlaylistLength(t, ml, playlistLibraryPath); length != len(playlist) {
		t.Fatalf("expected playlist length to be %v, got %v", len(playlist), length)
	}

	getPlaylistEntryShouldFail(t, ml, playlistLibraryPath, -1)
	getPlaylistEntryShouldFail(t, ml, playlistLibraryPath, len(playlist))

	for i := 0; i < len(playlist); i++ {
		entry := getPlaylistEntry(t, ml, playlistLibraryPath, i)

		var trackInfo struct {
			Name string
		}
		jsonBytes := simpleRequest(t, ml, "GET", entry.Path, "")
		unmarshalJson(t, jsonBytes, &trackInfo)

		expectedName := path.Base(playlist[i])
		if trackInfo.Name != expectedName {
			t.Errorf(
				"expected track name to be %q, got %q", expectedName, trackInfo.Name)
		}
	}
}

func TestStream(t *testing.T) {
	type TestInput struct {
		Paths        []string
		QueryStrings []string
	}

	inputArray := []TestInput{
		// test defaults with all files
		{
			Paths:        testFiles,
			QueryStrings: []string{""},
		},

		// test non-default settings with a single file
		{
			Paths: []string{"test.flac"},
			QueryStrings: combineQueryArgs([][]ArgTableEntry{
				{
					{"codec", []string{"mp3", "vorbis", "flac", "wav"}},
					{"sampleRate", []string{"", "22050"}},

					// https://ffmpeg.org/doxygen/4.0/samplefmt_8c_source.html#l00034
					{"sampleFormat", []string{"", "s16", "flt"}},

					// https://ffmpeg.org/doxygen/4.0/channel__layout_8c_source.html#l00075
					{"channelLayout", []string{"", "mono", "stereo", "5.1"}},
				},
				{
					{"codec", []string{"mp3", "vorbis"}},
					{"quality", []string{"3.5"}},
				},
				{
					{"codec", []string{"mp3", "vorbis"}},
					{"kbitRate", []string{"256"}},
				},
				{
					{"codec", []string{"foo"}},
				},
				{
					{"startTime", []string{"-1s", "2s", "2m5s"}},
				},
			}),
		},

		// ReplayGain only modifies audio data when gain is positive. Otherwise,
		// it is applied client-side.
		// (Positive gain isn't applied client-side because
		// HTMLAudioElement.volume can't be set greater than 1.0.)
		{
			Paths: []string{"test-positive-gain.ogg"},
			QueryStrings: combineQueryArgs([][]ArgTableEntry{
				{
					{"replayGain", []string{"track", "album", "off"}},
					{"preventClipping", []string{"true", "false"}},
				},
			}),
		},
	}

	ml := createDefaultLibrary(t)
	baselines := readBaselines()

	for _, input := range inputArray {
		for _, rangePath := range input.Paths {
			path := rangePath

			baselineHashes := baselines[path].StreamHashes
			if baselineHashes == nil {
				baselineHashes = make(map[string]string)

				baseline := baselines[path]
				baseline.StreamHashes = baselineHashes
				baselines[path] = baseline
			}

			for _, rangeQuery := range input.QueryStrings {
				query := rangeQuery

				t.Run(path+query, func(t *testing.T) {
					if !updateBaselines {
						t.Parallel()
					}

					uri := trackAt(path, "stream") + query
					body, statusCode := simpleRequestWithStatus(t, ml, "GET", uri, "")

					baselineHash := baselineHashes[query]

					if statusCode != http.StatusOK {
						if updateBaselines {
							baselineHashes[query] = "fail"
							return
						}

						if baselineHash == "fail" {
							return
						}
						t.Fatalf("expected %s, but request failed", baselineHash)
					}

					hash := sha256.New()
					if _, err := hash.Write(body); err != nil {
						t.Fatalf("hash.Write() failed: %v", err)
					}

					sum := hex.EncodeToString(hash.Sum(nil))

					if updateBaselines {
						baselineHashes[query] = sum
						return
					}

					if sum != baselineHash {
						t.Fatalf("expected %v, got %v", baselineHash, sum)
					}
				})
			}
		}
	}

	writeBaselines(baselines)
}

func TestTrackImages(t *testing.T) {
	ml := createDefaultLibrary(t)

	// Test non-existent file
	simpleRequestShouldFail(t, ml, "GET", trackAt("nonexistent.mp3", "images", "0"), "")

	// Test file with no images
	simpleRequestShouldFail(t, ml, "GET", trackAt("test.wav", "images", "9"), "")

	// Test invalid index
	simpleRequestShouldFail(t, ml, "GET", trackAt("test.flac", "images", "-1"), "")
	simpleRequestShouldFail(t, ml, "GET", trackAt("test.flac", "images", "abc"), "")
	simpleRequestShouldFail(t, ml, "GET", trackAt("test.flac", "images", "99"), "")

	// Test file with images - fetch track info first
	infoBody := simpleRequest(t, ml, "GET", trackAt("test.flac"), "")

	var trackInfo media.Track
	if err := json.Unmarshal(infoBody, &trackInfo); err != nil {
		t.Fatalf("failed to decode track info JSON: %v\n%s", err, indentJson(t, infoBody))
	}

	if len(trackInfo.AttachedImages) == 0 {
		t.Skip("test.flac has no attached images")
	}

	// Test each image index
	for i, imageInfo := range trackInfo.AttachedImages {
		t.Run(fmt.Sprintf("image_%d", i), func(t *testing.T) {
			// Verify URL field is present
			if imageInfo.Url == "" {
				t.Fatal("Expected image URL to be set")
			}

			uri := trackAt("test.flac", "images", fmt.Sprintf("%d", i))
			body, statusCode := simpleRequestWithStatus(t, ml, "GET", uri, "")

			if statusCode != http.StatusOK {
				t.Fatalf("Request failed with status %d: %s", statusCode, string(body))
			}

			// Check size
			actualSize := len(body)

			if actualSize != imageInfo.Size {
				t.Fatalf("Size mismatch for image %d: expected %d, got %d", i, imageInfo.Size, actualSize)
			}

			// Check content type
			w := httptest.NewRecorder()
			ml.ServeHTTP(w, httptest.NewRequest("GET", uri, nil))

			contentType := w.Header().Get("Content-Type")
			if contentType != imageInfo.MimeType {
				t.Fatalf("Content-Type mismatch for image %d: expected %s, got %s", i, imageInfo.MimeType, contentType)
			}

			// Verify hash-based URL returns the same image
			hashBody, hashStatus := simpleRequestWithStatus(t, ml, "GET", imageInfo.Url, "")
			if hashStatus != http.StatusOK {
				t.Fatalf("Hash URL request failed with status %d: %s", hashStatus, string(hashBody))
			}
			if !bytes.Equal(body, hashBody) {
				t.Fatalf("Hash URL returned different data than index URL")
			}
		})
	}
}

func TestTrackImageETag(t *testing.T) {
	ml := createDefaultLibrary(t)

	// Get track info to find images
	infoBody := simpleRequest(t, ml, "GET", trackAt("test.flac"), "")

	var trackInfo media.Track
	if err := json.Unmarshal(infoBody, &trackInfo); err != nil {
		t.Fatalf("failed to decode track info JSON: %v", err)
	}

	if len(trackInfo.AttachedImages) == 0 {
		t.Skip("test.flac has no attached images")
	}

	imageIndex := 0
	uri := trackAt("test.flac", "images", fmt.Sprintf("%d", imageIndex))

	// First request - should return image with ETag
	w1 := httptest.NewRecorder()
	ml.ServeHTTP(w1, httptest.NewRequest("GET", uri, nil))

	if w1.Code != http.StatusOK {
		t.Fatalf("First request failed with status %d", w1.Code)
	}

	etag := w1.Header().Get("ETag")
	if etag == "" {
		t.Fatal("Expected ETag header to be set")
	}

	// Second request with If-None-Match - should return 304
	req2 := httptest.NewRequest("GET", uri, nil)
	req2.Header.Set("If-None-Match", etag)
	w2 := httptest.NewRecorder()
	ml.ServeHTTP(w2, req2)

	if w2.Code != http.StatusNotModified {
		t.Fatalf("Expected 304 Not Modified, got %d", w2.Code)
	}

	// Third request with different If-None-Match - should return image
	req3 := httptest.NewRequest("GET", uri, nil)
	req3.Header.Set("If-None-Match", "\"different-etag\"")
	w3 := httptest.NewRecorder()
	ml.ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Fatalf("Third request failed with status %d", w3.Code)
	}
}

func TestHashImageRoute(t *testing.T) {
	ml := createDefaultLibrary(t)

	// Get a valid image URL from track info
	infoBody := simpleRequest(t, ml, "GET", trackAt("test.flac"), "")

	var trackInfo media.Track
	if err := json.Unmarshal(infoBody, &trackInfo); err != nil {
		t.Fatalf("failed to decode track info JSON: %v", err)
	}

	if len(trackInfo.AttachedImages) == 0 {
		t.Skip("test.flac has no attached images")
	}

	imageUrl := trackInfo.AttachedImages[0].Url

	// Valid hash URL should return 200
	w := httptest.NewRecorder()
	ml.ServeHTTP(w, httptest.NewRequest("GET", imageUrl, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	// Should have Cache-Control header
	cacheControl := w.Header().Get("Cache-Control")
	if cacheControl == "" {
		t.Fatal("Expected Cache-Control header to be set")
	}

	// Invalid hash should return 404
	simpleRequestShouldFail(t, ml, "GET", api("images", "hash:0000000000000000000000000000000000000000000000000000000000000000"), "")

	// Non-hex string should return 404
	simpleRequestShouldFail(t, ml, "GET", api("images", "hash:notahex"), "")

	// Missing hash: prefix should return 404
	simpleRequestShouldFail(t, ml, "GET", api("images", "something"), "")
}

func TestDir(t *testing.T) {
	ml := createDefaultLibrary(t)
	baselines := readBaselines()

	tests := []struct {
		name     string
		path     string
		baseline string
	}{
		{"root directory", "", "dirs.root"},
		{"subdirectory foo", "foo", "dirs.foo"},
		{"subdirectory with leading slash", "/foo", "dirs.foo"},
		{"subdirectory with trailing slash", "foo/", "dirs.foo"},
		{"subdirectory with leading and trailing slash", "/foo/", "dirs.foo"},
		{"nested subdirectory", "foo/another directory", "dirs.foo-another-directory"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			responseBody := simpleRequest(t, ml, "GET", dirAt(test.path), "")

			var dirInfo media.Dir
			if err := json.Unmarshal(responseBody, &dirInfo); err != nil {
				t.Fatalf("failed to decode dir info JSON: %v", err)
			}

			baseline, ok := baselines[test.baseline]
			if !ok && !updateBaselines {
				t.Fatalf("baseline %q not found", test.baseline)
			}

			expectedDirInfo := make(map[string]interface{})
			if err := json.Unmarshal(responseBody, &expectedDirInfo); err != nil {
				t.Fatalf("failed to decode response as map: %v", err)
			}

			// FFmpeg returns inconsistent values for "encoder" tag with .mka files
			if tracks, ok := expectedDirInfo["tracks"].([]interface{}); ok {
				for _, track := range tracks {
					removeJsonElement(track, "tags", "encoder")
				}
			}

			if updateBaselines {
				baseline.Info = expectedDirInfo
				baselines[test.baseline] = baseline
			} else if !reflect.DeepEqual(expectedDirInfo, baseline.Info) {
				t.Errorf("response doesn't match baseline for %q", test.baseline)
				t.Errorf("Expected: %+v", baseline.Info)
				t.Errorf("Got: %+v", expectedDirInfo)
			}
		})
	}

	if updateBaselines {
		writeBaselines(baselines)
	}
}

func TestSearch(t *testing.T) {
	ml := createDefaultLibrary(t)

	doSearch := func(query string) []media.SearchResult {
		t.Helper()
		uri := api("search") + "?q=" + url.QueryEscape(query)
		responseBody := simpleRequest(t, ml, "GET", uri, "")

		var response media.SearchResponse
		unmarshalJson(t, responseBody, &response)

		return response.Results
	}

	t.Run("EmptyQuery", func(t *testing.T) {
		results := doSearch("")
		if len(results) != 0 {
			t.Errorf("expected 0 results for empty query, got %d", len(results))
		}
	})

	t.Run("SearchForTracks", func(t *testing.T) {
		results := doSearch("test")
		if len(results) == 0 {
			t.Error("expected search for 'test' to return results")
		}
		for i, result := range results {
			if result.Type != "track" && result.Type != "dir" {
				t.Errorf("result %d has invalid type: %s", i, result.Type)
			}
			if result.Path == "" {
				t.Errorf("result %d missing 'path' field", i)
			}
			if result.URL == "" {
				t.Errorf("result %d missing 'url' field", i)
			}
		}
	})

	t.Run("SearchForSpecificFile", func(t *testing.T) {
		results := doSearch("test")

		found := false
		for _, result := range results {
			if result.Type == "track" && strings.HasSuffix(result.URL, "/tracks/at:test.mp3") {
				found = true
				break
			}
		}
		if !found {
			t.Error("search for 'test' should find test track file")
		}
	})

	t.Run("SearchForDirectory", func(t *testing.T) {
		results := doSearch("foo")

		found := false
		for _, result := range results {
			if result.Type == "dir" && strings.HasSuffix(result.URL, "/dirs/at:foo") {
				found = true
				break
			}
		}
		if !found {
			t.Error("search for 'foo' should find foo directory")
		}
	})

	t.Run("TrackResultsIncludeTrackInfo", func(t *testing.T) {
		results := doSearch("Aurelius Test Data")
		var found *media.SearchResult
		for i, result := range results {
			if result.Type == "track" && strings.HasSuffix(result.Path, "test.mp3") {
				found = &results[i]
				break
			}
		}
		if found == nil {
			t.Fatal("expected to find test.mp3 in search results via metadata")
		}
		if found.Track == nil {
			t.Fatal("expected track result to include track info")
		}
		if found.Track.Tags["title"] != "Aurelius Test Data" {
			t.Errorf("expected title tag 'Aurelius Test Data', got %q", found.Track.Tags["title"])
		}
		if found.Track.Tags["artist"] != "Aurelius" {
			t.Errorf("expected artist tag 'Aurelius', got %q", found.Track.Tags["artist"])
		}
		if found.Track.Tags["album"] != "Aurelius Test Data Greatest Hits" {
			t.Errorf("expected album tag 'Aurelius Test Data Greatest Hits', got %q", found.Track.Tags["album"])
		}
		if found.Track.Url == "" {
			t.Error("expected track info to include url")
		}
		if found.Track.Dir == "" {
			t.Error("expected track info to include dir")
		}
	})

	t.Run("DirectoryResultsHaveNoTrackInfo", func(t *testing.T) {
		results := doSearch("foo")
		for _, result := range results {
			if result.Type == "dir" && result.Track != nil {
				t.Errorf("directory result %q should not have track info", result.Path)
			}
		}
	})

	t.Run("SearchForNonexistent", func(t *testing.T) {
		results := doSearch("nonexistentfilename12345")
		if len(results) != 0 {
			t.Errorf("expected 0 results for nonexistent search, got %d", len(results))
		}
	})

	t.Run("CaseInsensitiveSearch", func(t *testing.T) {
		resultsLower := doSearch("test")
		resultsUpper := doSearch("TEST")
		resultsMixed := doSearch("Test")

		if len(resultsLower) == 0 {
			t.Error("lowercase search should return results")
		}
		if len(resultsUpper) == 0 {
			t.Error("uppercase search should return results")
		}
		if len(resultsMixed) == 0 {
			t.Error("mixed case search should return results")
		}
	})

	t.Run("PartialMatches", func(t *testing.T) {
		results := doSearch("test*")
		if len(results) == 0 {
			t.Error("search for 'test*' should find files starting with 'test'")
		}
	})

	t.Run("SearchForToken", func(t *testing.T) {
		results := doSearch("Aurelius Test Data")
		if len(results) == 0 {
			t.Error("search for 'Aurelius Test Data' should find tracks via metadata")
		}
	})

	t.Run("SearchByArtist", func(t *testing.T) {
		results := doSearch("Aurelius")
		if len(results) == 0 {
			t.Error("search for 'Aurelius' should find tracks by artist tag")
		}
		foundTrack := false
		for _, result := range results {
			if result.Type == "track" {
				foundTrack = true
				break
			}
		}
		if !foundTrack {
			t.Error("search for 'Aurelius' should return at least one track result")
		}
	})

	t.Run("SearchByTitle", func(t *testing.T) {
		results := doSearch("Baz All Night")
		found := false
		for _, result := range results {
			if result.Type == "track" && result.Track != nil && result.Track.Tags["title"] == "Baz All Night" {
				found = true
				break
			}
		}
		if !found {
			t.Error("search for 'Baz All Night' should find the fragment track by title tag")
		}
	})

	t.Run("FragmentSourceFilesExcluded", func(t *testing.T) {
		results := doSearch("test.flac")
		for _, result := range results {
			if result.Type == "track" && strings.HasSuffix(result.Path, "test.flac") {
				t.Error("fragment source file test.flac should not appear in search results")
			}
		}
	})

	t.Run("SearchByAlbum", func(t *testing.T) {
		results := doSearch("Greatest Hits")
		if len(results) == 0 {
			t.Error("search for 'Greatest Hits' should find tracks by album tag")
		}
		foundTrack := false
		for _, result := range results {
			if result.Type == "track" {
				foundTrack = true
				break
			}
		}
		if !foundTrack {
			t.Error("search for 'Greatest Hits' should return at least one track result")
		}
	})

	t.Run("DirOnlyModifier", func(t *testing.T) {
		results := doSearch(".d foo")
		if len(results) == 0 {
			t.Fatal("expected results for '.d foo'")
		}
		for _, result := range results {
			if result.Type != "dir" {
				t.Errorf("expected only dir results with .d modifier, got type %q for %q", result.Type, result.Path)
			}
		}
	})

	t.Run("DirOnlyModifierAtEnd", func(t *testing.T) {
		results := doSearch("foo .d")
		if len(results) == 0 {
			t.Fatal("expected results for 'foo .d'")
		}
		for _, result := range results {
			if result.Type != "dir" {
				t.Errorf("expected only dir results with .d modifier at end, got type %q", result.Type)
			}
		}
	})

	t.Run("TracksOnlyModifier", func(t *testing.T) {
		results := doSearch(".t foo")
		if len(results) == 0 {
			t.Fatal("expected results for '.t foo'")
		}
		for _, result := range results {
			if result.Type != "track" {
				t.Errorf("expected only track results with .t modifier, got type %q for %q", result.Type, result.Path)
			}
		}
	})

	t.Run("DirAndTrackModifiersEmpty", func(t *testing.T) {
		results := doSearch(".d .t foo")
		if len(results) != 0 {
			t.Errorf("expected 0 results with both .d and .t modifiers, got %d", len(results))
		}
	})

	t.Run("FavoritesOnlyModifier", func(t *testing.T) {
		// Favorite a track first.
		favPath := testFiles[1]
		simpleRequest(t, ml, "POST", trackAt(favPath, "favorite"), "")

		results := doSearch(".f test")
		if len(results) == 0 {
			t.Fatal("expected results for '.f test' after favoriting a track")
		}
		for _, result := range results {
			if result.Type != "track" {
				t.Errorf("expected only track results with .f modifier, got type %q", result.Type)
			}
			if result.Track == nil || !result.Track.Favorite {
				t.Errorf("expected only favorite tracks with .f modifier, got non-favorite %q", result.Path)
			}
		}

		// Clean up.
		simpleRequest(t, ml, "POST", trackAt(favPath, "unfavorite"), "")
	})

	t.Run("BothModifiersEmpty", func(t *testing.T) {
		results := doSearch(".d .f test")
		if len(results) != 0 {
			t.Errorf("expected 0 results with both .d and .f modifiers, got %d", len(results))
		}
	})

	t.Run("ModifierAloneNoQuery", func(t *testing.T) {
		results := doSearch(".d")
		if len(results) != 0 {
			t.Errorf("expected 0 results for modifier alone with no search terms, got %d", len(results))
		}
	})
}

func TestPlayHistory(t *testing.T) {
	ml := createDefaultLibrary(t)
	db := ml.DB()

	trackPath := "test.flac"

	// Streaming from the beginning (no startTime) should record a play.
	simpleRequest(t, ml, "GET", trackAt(trackPath, "stream"), "")

	count, err := db.PlayCount(trackPath)
	if err != nil {
		t.Fatalf("PlayCount failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 play after streaming from beginning, got %d", count)
	}

	// Streaming with startTime=0 should also record a play.
	simpleRequest(t, ml, "GET", trackAt(trackPath, "stream")+"?startTime=0s", "")

	count, err = db.PlayCount(trackPath)
	if err != nil {
		t.Fatalf("PlayCount failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 plays after streaming with startTime=0, got %d", count)
	}

	// Streaming with a non-zero startTime should NOT record a play.
	simpleRequest(t, ml, "GET", trackAt(trackPath, "stream")+"?startTime=2s", "")

	count, err = db.PlayCount(trackPath)
	if err != nil {
		t.Fatalf("PlayCount failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 plays after streaming with startTime=2s, got %d", count)
	}
}
