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
	"runtime"
	"strings"
	"testing"

	"github.com/beakbeak/aurelius/pkg/media"
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
		"test.flac.1.txt",
		"test.flac.2.txt",
		"test.flac.3.txt",
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

type PlaylistEntry struct {
	Path string
	Pos  int
}

func getPlaylistEntry(
	t *testing.T,
	ml *media.Library,
	path string,
	pos int,
) PlaylistEntry {
	t.Helper()
	url := fmt.Sprintf("%s/tracks/%v", path, pos)
	jsonBytes := simpleRequest(t, ml, "GET", url, "")

	var entry PlaylistEntry
	unmarshalJson(t, jsonBytes, &entry)
	return entry
}

func getPlaylistEntryWithPrefix(
	t *testing.T,
	ml *media.Library,
	basePath string,
	pos int,
	prefix string,
) PlaylistEntry {
	t.Helper()
	url := fmt.Sprintf("%s/tracks/%v?prefix=%s", basePath, pos, prefix)
	jsonBytes := simpleRequest(t, ml, "GET", url, "")

	var entry PlaylistEntry
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

type PathUrl struct {
	Name, Url string
}

type DirInfo struct {
	Path                    string
	Dirs, Playlists, Tracks []PathUrl
}

func getDirInfo(
	t *testing.T,
	ml *media.Library,
	path string,
) DirInfo {
	t.Helper()
	jsonBytes := simpleRequest(t, ml, "GET", path, "")

	var info DirInfo
	unmarshalJson(t, jsonBytes, &info)
	return info
}

// Tests ///////////////////////////////////////////////////////////////////////

func TestTrackInfo(t *testing.T) {
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

	path := testFiles[0]

	simpleRequest(t, ml, "POST", trackAt(path, "unfavorite"), "")

	if isFavorite(t, ml, path) {
		t.Fatalf("expected 'favorite' to be false for \"%s\"", path)
	}

	simpleRequest(t, ml, "POST", trackAt(path, "favorite"), "")

	if !isFavorite(t, ml, path) {
		t.Fatalf("expected 'favorite' to be true for \"%s\"", path)
	}

	simpleRequest(t, ml, "POST", trackAt(path, "unfavorite"), "")

	if isFavorite(t, ml, path) {
		t.Fatalf("expected 'favorite' to be false for \"%s\"", path)
	}
}

func TestFavoritePaths(t *testing.T) {
	ml := createDefaultLibrary(t)

	simpleRequestShouldFail(t, ml, "GET", api("playlists", "favorites"), "")

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

	// Fetch and compare the paths of favorites
	favoritesPath := api("playlists", "favorites")
	var actualPaths []string
	for i := 0; i < length; i++ {
		entry := getPlaylistEntry(t, ml, favoritesPath, i)
		actualPaths = append(actualPaths, entry.Path)
	}

	// Build expected paths
	var expectedPaths []string
	for _, path := range testFiles {
		expectedPaths = append(expectedPaths, trackAt(path))
	}

	// Compare paths
	if len(actualPaths) != len(expectedPaths) {
		t.Fatalf("expected %v paths, got %v", len(expectedPaths), len(actualPaths))
	}

	for i, expectedPath := range expectedPaths {
		if actualPaths[i] != expectedPath {
			t.Errorf("path mismatch at index %v: expected %q, got %q", i, expectedPath, actualPaths[i])
		}
	}

	// Test prefix filtering
	t.Run("PrefixFiltering", func(t *testing.T) {
		// Test with "test.flac" prefix - should match test.flac and test.flac.*.txt files
		prefixLength := getPlaylistLength(t, ml, api("playlists", "favorites")+"?prefix=test.flac")
		expectedPrefixLength := 4 // test.flac + test.flac.1.txt + test.flac.2.txt + test.flac.3.txt
		if prefixLength != expectedPrefixLength {
			t.Errorf("expected prefix 'test.flac' to match %v entries, got %v", expectedPrefixLength, prefixLength)
		}

		// Test with "foo/" prefix - should match foo/another directory/test.m4a
		prefixLength = getPlaylistLength(t, ml, api("playlists", "favorites")+"?prefix=foo/")
		expectedPrefixLength = 1
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
		testPrefix := "test.flac"
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

func TestWithSymlinks(t *testing.T) {
	for _, baseName := range []string{"dir1", "dir2"} {
		dir := filepath.Join(testMediaPath, baseName)
		if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
			t.Fatalf("RemoveAll(\"%s\") failed: %v", dir, err)
		}
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			t.Fatalf("MkdirAll(\"%s\") failed: %v", dir, err)
		}

		defer (func() {
			if err := os.RemoveAll(dir); err != nil {
				t.Logf("RemoveAll(\"%s\") failed: %v", dir, err)
				t.Fail()
			}
		})()
	}

	useSymlinks := true
	{
		linkTarget := filepath.Join("..", "dir1")
		linkName := filepath.Join(testMediaPath, "dir2", "dir1link")

		if err := os.Symlink(linkTarget, linkName); err != nil {
			t.Logf("Symlink(\"%s\", \"%s\") failed: %v", linkTarget, linkName, err)

			if runtime.GOOS == "windows" {
				t.Log("assuming symlinks aren't supported")
				useSymlinks = false
			} else {
				t.FailNow()
			}
		}
	}

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

	if useSymlinks {
		baseNames := []string{
			"test #1.flac",
			"test 2? yes!.flac",
			"test 3: $p€c¡&l character edition.flac",
		}

		linkTarget := filepath.Join("..", "test.flac")
		for _, baseName := range baseNames {
			linkName := filepath.Join(testMediaPath, "dir1", baseName)
			if err := os.Symlink(linkTarget, linkName); err != nil {
				t.Fatalf("Symlink(\"%s\", \"%s\") failed: %v", linkTarget, linkName, err)
			}

			playlist = append(
				playlist,
				filepath.Join("dir1", baseName),
				filepath.Join("dir2", "dir1link", baseName),
			)
		}
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

	t.Run("Playlist", func(t *testing.T) {
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
					"expected track name to be \"%s\", got \"%s\"", expectedName, trackInfo.Name)
			}
		}
	})

	// Test prefix filtering for M3U playlists
	t.Run("PlaylistPrefixFiltering", func(t *testing.T) {
		testPrefix := "test.f"

		// Test with "test.flac" prefix
		prefixLength := getPlaylistLength(t, ml, playlistLibraryPath+"?prefix="+testPrefix)
		expectedCount := 0
		for _, item := range playlist {
			if strings.HasPrefix(item, "test.flac") {
				expectedCount++
			}
		}
		if prefixLength != expectedCount {
			t.Errorf("expected prefix 'test.flac' to match %v entries, got %v", expectedCount, prefixLength)
		}

		// Test fetching entries with prefix
		for i := 0; i < prefixLength; i++ {
			entry := getPlaylistEntryWithPrefix(t, ml, playlistLibraryPath, i, testPrefix)
			if entry.Path == "" {
				t.Errorf("expected non-empty path for prefix entry %d", i)
			}
			if entry.Pos != i {
				t.Errorf("expected position %d for prefix entry, got %d", i, entry.Pos)
			}
		}
	})

	t.Run("Directory listing", func(t *testing.T) {
		var testDir func(string)

		testDir = func(path string) {
			info := getDirInfo(t, ml, path)
			if info.Path == "" {
				t.Errorf("want non-empty path, got %q", path)
			}

			for _, playlistUrl := range info.Playlists {
				getPlaylistLength(t, ml, playlistUrl.Url)
			}

			for _, trackUrl := range info.Tracks {
				var trackInfo struct {
					Name string
				}
				jsonBytes := simpleRequest(t, ml, "GET", trackUrl.Url, "")
				unmarshalJson(t, jsonBytes, &trackInfo)

				expectedName := trackUrl.Name
				if trackInfo.Name != expectedName {
					t.Errorf(
						"expected track name to be \"%s\", got \"%s\"",
						expectedName, trackInfo.Name)
				}
			}

			for _, dirUrl := range info.Dirs {
				testDir(dirUrl.Url)
			}
		}

		testDir(dirAt("/"))
	})
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

	type AttachedImageInfo struct {
		MimeType string `json:"mimeType"`
		Size     int    `json:"size"`
	}
	type TrackInfo struct {
		AttachedImages []AttachedImageInfo `json:"attachedImages"`
	}
	var trackInfo TrackInfo
	if err := json.Unmarshal(infoBody, &trackInfo); err != nil {
		t.Fatalf("failed to decode track info JSON: %v\n%s", err, indentJson(t, infoBody))
	}

	if len(trackInfo.AttachedImages) == 0 {
		t.Skip("test.flac has no attached images")
	}

	// Test each image index
	for i, imageInfo := range trackInfo.AttachedImages {
		t.Run(fmt.Sprintf("image_%d", i), func(t *testing.T) {
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
		})
	}
}

func TestTrackImageETag(t *testing.T) {
	ml := createDefaultLibrary(t)

	// Get track info to find images
	infoBody := simpleRequest(t, ml, "GET", trackAt("test.flac"), "")

	type AttachedImageInfo struct {
		Size int `json:"size"`
	}
	type TrackInfo struct {
		AttachedImages []AttachedImageInfo `json:"attachedImages"`
	}
	var trackInfo TrackInfo
	if err := json.Unmarshal(infoBody, &trackInfo); err != nil {
		t.Fatalf("failed to decode track info JSON: %v", err)
	}

	if len(trackInfo.AttachedImages) == 0 {
		t.Skip("test.flac has no attached images")
	}

	imageIndex := 0
	uri := trackAt("test.flac", "images", fmt.Sprintf("%d", imageIndex))
	expectedSize := trackInfo.AttachedImages[imageIndex].Size

	// First request - should return image with ETag
	w1 := httptest.NewRecorder()
	ml.ServeHTTP(w1, httptest.NewRequest("GET", uri, nil))

	if w1.Code != http.StatusOK {
		t.Fatalf("First request failed with status %d", w1.Code)
	}

	etag := w1.Header().Get("ETag")
	expectedETag := fmt.Sprintf("\"%x\"", expectedSize)
	if etag != expectedETag {
		t.Fatalf("ETag mismatch: expected %s, got %s", expectedETag, etag)
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

func TestDirInfo(t *testing.T) {
	ml := createDefaultLibrary(t)
	baselines := readBaselines()

	tests := []struct {
		name     string
		path     string
		baseline string
	}{
		{"root directory", "/", "dirs.root"},
		{"subdirectory foo", "/foo", "dirs.foo"},
		{"nested subdirectory", "/foo/another directory", "dirs.foo-another-directory"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			responseBody := simpleRequest(t, ml, "GET", dirAt(test.path), "")

			type PathUrl struct {
				Name string `json:"name"`
				Url  string `json:"url"`
			}
			type DirInfo struct {
				TopLevel  string    `json:"topLevel"`
				Parent    string    `json:"parent"`
				Path      string    `json:"path"`
				Dirs      []PathUrl `json:"dirs"`
				Playlists []PathUrl `json:"playlists"`
				Tracks    []PathUrl `json:"tracks"`
			}

			var dirInfo DirInfo
			if err := json.Unmarshal(responseBody, &dirInfo); err != nil {
				t.Fatalf("failed to decode dir info JSON: %v", err)
			}

			// Verify basic structure
			if dirInfo.Path != test.path {
				t.Errorf("expected path %q, got %q", test.path, dirInfo.Path)
			}

			// Check baseline
			baseline, ok := baselines[test.baseline]
			if !ok && !updateBaselines {
				t.Fatalf("baseline %q not found", test.baseline)
			}

			expectedDirInfo := make(map[string]interface{})
			if err := json.Unmarshal(responseBody, &expectedDirInfo); err != nil {
				t.Fatalf("failed to decode response as map: %v", err)
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
