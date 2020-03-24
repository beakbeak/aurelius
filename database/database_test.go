package database_test

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sb/aurelius/database"
	"strings"
	"testing"
)

type testFileInfo struct {
	path string
	json string
}

var testFiles = []testFileInfo{
	{"test.flac", `{
		"name": "test.flac",
		"duration": 12.59932,
		"replayGainTrack": 0.7507580541199371,
		"replayGainAlbum": 0.7507580541199371,
		"favorite": false,
		"tags": {
			"album": "Aurelius Test Data Greatest Hits",
			"artist": "Aurelius",
			"comment": "Testing",
			"composer": "J. S. Bach",
			"date": "2020",
			"genre": "Test Data",
			"replaygain_album_gain": "-2.49 dB",
			"replaygain_album_peak": "0.89123535",
			"replaygain_reference_loudness": "89.0 dB",
			"replaygain_track_gain": "-2.49 dB",
			"replaygain_track_peak": "0.89123535",
			"title": "Aurelius Test Data",
			"track": "1"
		}
	}`},
	{"test.mp3", `{
		"name": "test.mp3",
		"duration": 12.669388,
		"replayGainTrack": 0.716143410212902,
		"replayGainAlbum": 0.716143410212902,
		"favorite": false,
		"tags": {
			"album": "Aurelius Test Data Greatest Hits",
			"artist": "Aurelius",
			"comment": "Testing",
			"composer": "J. S. Bach",
			"date": "2020",
			"encoder": "LAME3.100",
			"genre": "Test Data",
			"title": "Aurelius Test Data",
			"track": "1"
		}
	}`},
	{"test.ogg", `{
		"name": "test.ogg",
		"duration": 12.59932,
		"replayGainTrack": 0.7612021390057184,
		"replayGainAlbum": 0.7612021390057184,
		"favorite": false,
		"tags": {
			"album": "Aurelius Test Data Greatest Hits",
			"artist": "Aurelius",
			"comment": "Testing",
			"composer": "J. S. Bach",
			"date": "2020",
			"genre": "Test Data",
			"replaygain_album_gain": "-2.37 dB",
			"replaygain_album_peak": "0.85044265",
			"replaygain_track_gain": "-2.37 dB",
			"replaygain_track_peak": "0.85044265",
			"title": "Aurelius Test Data",
			"track": "1"
		}
	}`},
	{"test.wav", `{
		"name": "test.wav",
		"duration": 12.59932,
		"replayGainTrack": 1,
		"replayGainAlbum": 1,
		"favorite": false,
		"tags": {
			"album": "Aurelius Test Data Greatest Hits",
			"artist": "Aurelius",
			"comment": "Testing",
			"date": "2020",
			"genre": "Test Data",
			"title": "Aurelius Test Data",
			"track": "1"
		}
	}`},
}

func createDefaultDatabase(t *testing.T) *database.Database {
	db, err := database.New("/db", "../test-data", "../cmd/aurelius/html")
	if err != nil {
		t.Fatalf("failed to create Database: %v", err)
	}
	return db
}

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
	return string(buf.Bytes())
}

func simpleRequest(
	t *testing.T,
	db *database.Database,
	method string,
	path string,
	requestBody string,
) (*http.Response, []byte) {
	w := httptest.NewRecorder()
	db.ServeHTTP(w, httptest.NewRequest(method, path, strings.NewReader(requestBody)))
	response := w.Result()

	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if response.StatusCode != 200 {
		t.Fatalf("GET '%s' failed with code %v:\n%v", path, response.StatusCode, responseBody)
	}
	return response, responseBody
}

func TestTrackInfo(t *testing.T) {
	db := createDefaultDatabase(t)

	for _, fileInfo := range testFiles {
		t.Run(fileInfo.path, func(t *testing.T) {
			response, body := simpleRequest(t, db, "GET", "/db/"+fileInfo.path+"/info", "")

			contentType := response.Header["Content-Type"]
			if len(contentType) != 1 || contentType[0] != "application/json" {
				t.Fatalf("unexpected Content-Type: %v", contentType)
			}

			expectedBody := []byte(fileInfo.json)
			if !jsonEqual(t, body, expectedBody) {
				t.Fatalf("unexpected JSON: %v", indentJson(t, body))
			}
		})
	}
}

func isFavorite(
	t *testing.T,
	db *database.Database,
	path string,
) bool {
	_, bodyBytes := simpleRequest(t, db, "GET", "/db/"+path+"/info", "")

	var bodyJson struct{ Favorite bool }
	err := json.Unmarshal(bodyBytes, &bodyJson)
	if err != nil {
		t.Fatalf("json.Unmarshal() failed: %v\n%s", err, string(bodyBytes))
	}

	expectedValue, err := db.IsFavorite(path)
	if err != nil {
		t.Fatalf("db.IsFavorite(\"%s\") failed: %v", path, err)
	}

	if expectedValue != bodyJson.Favorite {
		t.Fatalf("db.IsFavorite(\"%s\") doesn't match track info", path)
	}
	return bodyJson.Favorite
}

func TestFavorite(t *testing.T) {
	db := createDefaultDatabase(t)
	path := testFiles[0].path

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
