package database_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"sb/aurelius/database"
	"strings"
	"testing"
)

var (
	// After confirming new test results are correct, set to true and re-run
	// tests to update baseline.json
	updateBaselines = os.Getenv("UPDATE_BASELINES") == "1"

	testDataPath     = filepath.Join("..", "test-data")
	testDataDbPath   = filepath.Join(testDataPath, "db")
	baselineJsonPath = filepath.Join(testDataPath, "baseline.json")

	favoritesDbPath   = "/db/Favorites.m3u"
	favoritesFilePath = filepath.Join(testDataDbPath, "Favorites.m3u")

	htmlPath = filepath.Join("..", "cmd", "aurelius")

	testFiles = []string{
		"test.flac",
		"test.mp3",
		"test.ogg",
		"test.wav",
		"test.flac.1.txt",
		"test.flac.2.txt",
		"test.flac.3.txt",
	}
)

/* Baselines ******************************************************************/

type Baseline struct {
	TrackInfo    map[string]interface{} // result of "/info" request
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

/* General utilities **********************************************************/

func simpleRequest(
	t *testing.T,
	handler http.Handler,
	method string,
	path string,
	requestBody string,
) []byte {
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(requestBody)))
	response := w.Result()
	defer response.Body.Close()

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if response.StatusCode != 200 {
		t.Fatalf(
			"%s '%s' failed with code %v:\n%v",
			method, path, response.StatusCode, string(responseBody))
	}
	return responseBody
}

func simpleRequestShouldFail(
	t *testing.T,
	handler http.Handler,
	method string,
	path string,
	requestBody string,
) {
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(requestBody)))
	response := w.Result()
	defer response.Body.Close()

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if response.StatusCode == 200 {
		t.Fatalf("%s '%s' succeeded but should have failed:\n%v", method, path, responseBody)
	}
}

func writeStringToFile(
	t *testing.T,
	path string,
	contents string,
) {
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

/* JSON utilities *************************************************************/

func jsonEqual(
	t *testing.T,
	s1 []byte,
	s2 []byte,
) bool {
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
	err := json.Unmarshal(data, v)
	if err != nil {
		t.Fatalf("json.Unmarshal() failed: %v\n%s", err, string(data))
	}
}

/* Database utilities *********************************************************/

func createDefaultDatabase(t *testing.T) *database.Database {
	clearFavorites(t)

	db, err := database.New("/db", testDataDbPath, htmlPath)
	if err != nil {
		t.Fatalf("failed to create Database: %v", err)
	}
	return db
}

func clearFavorites(t *testing.T) {
	switch _, err := os.Stat(favoritesFilePath); {
	case os.IsNotExist(err):
		return
	case err != nil:
		t.Fatalf("\"%s\" exists but Stat() failed: %v", favoritesFilePath, err)
	}

	if err := os.Remove(favoritesFilePath); err != nil {
		t.Fatalf("Remove(\"%s\") failed: %v", favoritesFilePath, err)
	}
}

func isFavorite(
	t *testing.T,
	db *database.Database,
	path string,
) bool {
	jsonBytes := simpleRequest(t, db, "GET", "/db/"+path+"/info", "")

	var track struct{ Favorite bool }
	unmarshalJson(t, jsonBytes, &track)

	expectedValue, err := db.IsFavorite(path)
	if err != nil {
		t.Fatalf("db.IsFavorite(\"%s\") failed: %v", path, err)
	}

	if expectedValue != track.Favorite {
		t.Fatalf("db.IsFavorite(\"%s\") doesn't match track info", path)
	}
	return track.Favorite
}

type PlaylistEntry struct {
	Path string
	Pos  int
}

func getPlaylistEntry(
	t *testing.T,
	handler http.Handler,
	path string,
	pos int,
) PlaylistEntry {
	url := fmt.Sprintf("%s?pos=%v", path, pos)
	jsonBytes := simpleRequest(t, handler, "GET", url, "")

	var entry PlaylistEntry
	unmarshalJson(t, jsonBytes, &entry)
	return entry
}

func getPlaylistEntryShouldFail(
	t *testing.T,
	handler http.Handler,
	path string,
	pos int,
) {
	url := fmt.Sprintf("%s?pos=%v", path, pos)
	jsonBytes := simpleRequest(t, handler, "GET", url, "")

	if !jsonEqual(t, jsonBytes, []byte("null")) {
		t.Fatalf("expected %s to be null, got %v", url, string(jsonBytes))
	}
}

func getPlaylistLength(
	t *testing.T,
	handler http.Handler,
	dbPath string,
) int {
	jsonBytes := simpleRequest(t, handler, "GET", dbPath, "")

	var playlist struct{ Length int }
	unmarshalJson(t, jsonBytes, &playlist)

	return playlist.Length
}

type PathUrl struct {
	Name, Url string
}

type DirInfo struct {
	Dirs, Playlists, Tracks []PathUrl
}

func getDirInfo(
	t *testing.T,
	handler http.Handler,
	path string,
) DirInfo {
	jsonBytes := simpleRequest(t, handler, "GET", path+"/?info", "")

	var info DirInfo
	unmarshalJson(t, jsonBytes, &info)
	return info
}

/* Tests **********************************************************************/

func TestTrackInfo(t *testing.T) {
	db := createDefaultDatabase(t)

	simpleRequestShouldFail(t, db, "GET", "/db/nonexistent.mp3/info", "")

	baselines := readBaselines()

	for _, rangePath := range testFiles {
		path := rangePath

		t.Run(path, func(t *testing.T) {
			body := simpleRequest(t, db, "GET", "/db/"+path+"/info", "")

			var trackInfo map[string]interface{}
			if err := json.Unmarshal(body, &trackInfo); err != nil {
				t.Fatalf("failed to decode JSON: %v\n%s", err, indentJson(t, body))
			}

			baseline := baselines[path]

			if updateBaselines {
				baseline.TrackInfo = trackInfo
				baselines[path] = baseline
				return
			}

			if !reflect.DeepEqual(trackInfo, baseline.TrackInfo) {
				t.Fatalf("unexpected JSON: %v", indentJson(t, body))
			}
		})
	}

	if updateBaselines {
		writeBaselines(baselines)
	}
}

func TestFavorite(t *testing.T) {
	db := createDefaultDatabase(t)

	simpleRequestShouldFail(t, db, "POST", "/db/nonexistent.mp3/favorite", "")
	simpleRequestShouldFail(t, db, "POST", "/db/nonexistent.mp3/unfavorite", "")

	path := testFiles[0]

	simpleRequest(t, db, "POST", "/db/"+path+"/unfavorite", "")

	if isFavorite(t, db, path) {
		t.Fatalf("expected 'favorite' to be false for \"%s\"", path)
	}

	simpleRequest(t, db, "POST", "/db/"+path+"/favorite", "")

	if !isFavorite(t, db, path) {
		t.Fatalf("expected 'favorite' to be true for \"%s\"", path)
	}

	simpleRequest(t, db, "POST", "/db/"+path+"/unfavorite", "")

	if isFavorite(t, db, path) {
		t.Fatalf("expected 'favorite' to be false for \"%s\"", path)
	}
}

func TestFavoritesLength(t *testing.T) {
	db := createDefaultDatabase(t)

	simpleRequestShouldFail(t, db, "GET", favoritesDbPath, "")

	for i := 0; i < 2; i++ {
		for _, path := range testFiles {
			simpleRequest(t, db, "POST", "/db/"+path+"/favorite", "")
		}
	}

	length := getPlaylistLength(t, db, favoritesDbPath)
	expectedLength := len(testFiles)
	if length != expectedLength {
		t.Fatalf("expected favorites to have %v entries, got %v", expectedLength, length)
	}
}

func TestWithSymlinks(t *testing.T) {
	for _, baseName := range []string{"dir1", "dir2"} {
		dir := filepath.Join(testDataDbPath, baseName)
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
		linkName := filepath.Join(testDataDbPath, "dir2", "dir1link")

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
			linkName := filepath.Join(testDataDbPath, "dir1", baseName)
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

	playlistPath := filepath.Join(testDataDbPath, "playlist.m3u")
	writeStringToFile(t, playlistPath, strings.Join(playlist, "\n"))
	defer (func() {
		if err := os.Remove(playlistPath); err != nil {
			t.Logf("Remove(\"%s\") failed: %v", playlistPath, err)
			t.Fail()
		}
	})()

	db := createDefaultDatabase(t)

	t.Run("Playlist", func(t *testing.T) {
		if length := getPlaylistLength(t, db, "/db/playlist.m3u"); length != len(playlist) {
			t.Fatalf("expected playlist length to be %v, got %v", len(playlist), length)
		}

		getPlaylistEntryShouldFail(t, db, "/db/playlist.m3u", -1)
		getPlaylistEntryShouldFail(t, db, "/db/playlist.m3u", len(playlist))

		for i := 0; i < len(playlist); i++ {
			entry := getPlaylistEntry(t, db, "/db/playlist.m3u", i)

			var trackInfo struct {
				Name string
			}
			jsonBytes := simpleRequest(t, db, "GET", entry.Path+"/info", "")
			unmarshalJson(t, jsonBytes, &trackInfo)

			expectedName := path.Base(playlist[i])
			if trackInfo.Name != expectedName {
				t.Fatalf(
					"expected track name to be \"%s\", got \"%s\"", expectedName, trackInfo.Name)
			}
		}
	})

	t.Run("Directory listing", func(t *testing.T) {
		var testDir func(string)

		testDir = func(path string) {
			info := getDirInfo(t, db, path)

			for _, playlistUrl := range info.Playlists {
				getPlaylistLength(t, db, playlistUrl.Url)
			}

			for _, trackUrl := range info.Tracks {
				var trackInfo struct {
					Name string
				}
				jsonBytes := simpleRequest(t, db, "GET", trackUrl.Url+"/info", "")
				unmarshalJson(t, jsonBytes, &trackInfo)

				expectedName := trackUrl.Name
				if trackInfo.Name != expectedName {
					t.Fatalf(
						"expected track name to be \"%s\", got \"%s\"",
						expectedName, trackInfo.Name)
				}
			}

			for _, dirUrl := range info.Dirs {
				testDir(dirUrl.Url)
			}
		}

		testDir("/db")
	})
}
