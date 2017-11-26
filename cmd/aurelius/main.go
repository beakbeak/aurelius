package main

// TODO:
// - REST(?) API
//   - support embedded images
// - CLI client
// - implement pause (save current source, set it to SilenceSource; restore saved on unpause)
// - server configuration
//   - make ReplayGain mode configurable
//   - basic playlist management
// - seeking
// - combine artists/etc. with different capitalizations/whitespace/accents?/brackets/etc.

// WISHLIST:
// - replaygain preamp?
// - treat sections of a file as playlist entries (e.g. pieces of a long live set, a hidden track)
//   - can't use m3u anymore
// - tag editing
// - get replaygain info from RVA2 mp3 tag (requires another library dependency)
// - figure out why timing is wrong when using MKV container
//   (maybe something to do with time base settings/conversion)
// - direct audio output

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sb/aurelius/aurelib"
)

// TODO: make configurable
const maxBufferedFrames = 32

var debug *log.Logger
var noise *log.Logger
var player *Player

func main() {
	if len(os.Args) < 2 {
		panic("not enough arguments")
	}

	debug = log.New(os.Stdout, "DEBUG: ", log.Ltime|log.Lmicroseconds|log.Ldate|log.Lshortfile)
	noise = log.New(os.Stdout, "NOISE: ", log.Ltime|log.Lmicroseconds|log.Ldate|log.Lshortfile)

	noise.SetOutput(ioutil.Discard)
	noise.SetFlags(0)

	aurelib.NetworkInit()
	defer aurelib.NetworkDeinit()

	player = NewPlayer()
	defer player.Destroy()

	player.Play(NewFilePlaylist(os.Args[1:]))

	debug.Println("waiting for connections")
	http.HandleFunc("/stream", stream)
	log.Fatal(http.ListenAndServe(":9090", nil))
}