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
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sb/aurelius/aurelib"

	"github.com/gorilla/mux"
)

// TODO: make configurable
const maxBufferedFrames = 32

var debugEnabled = true
var debug *log.Logger
var noise *log.Logger

func init() {
	debug = log.New(os.Stdout, "DEBUG: ", log.Ltime|log.Lmicroseconds|log.Ldate|log.Lshortfile)
	noise = log.New(os.Stdout, "NOISE: ", log.Ltime|log.Lmicroseconds|log.Ldate|log.Lshortfile)
}

var player *Player

func main() {
	var (
		address = flag.String(
			"address", "", "address at which to listen for connections; overrides port setting")
		port     = flag.Int("port", 9090, "port on which to listen for connections")
		cert     = flag.String("cert", "", "TLS certificate file")
		key      = flag.String("key", "", "TLS key file")
		logLevel = flag.Int("log", 2, "log verbosity (1-3)")
	)
	flag.Parse()

	if *logLevel < 2 {
		debugEnabled = false
		debug.SetOutput(ioutil.Discard)
		debug.SetFlags(0)
	}
	if *logLevel < 3 {
		noise.SetOutput(ioutil.Discard)
		noise.SetFlags(0)
	}
	if *logLevel > 1 {
		aurelib.SetLogLevel(aurelib.LogInfo)
	}

	if len(*address) == 0 {
		*address = fmt.Sprintf(":%v", *port)
	}

	aurelib.NetworkInit()
	defer aurelib.NetworkDeinit()

	player = NewPlayer()
	defer player.Destroy()

	router := mux.NewRouter()
	router.HandleFunc("/stream", stream).Methods("GET")
	router.HandleFunc("/rpc", dispatchRpc).
		Methods("POST").
		Headers("Content-Type", "application/json")
	http.Handle("/", router)

	player.Play(NewFilePlaylist(flag.Args()))

	log.Printf("listening on %s\n", *address)
	if len(*cert) > 0 || len(*key) > 0 {
		log.Printf("using HTTPS")
		log.Fatal(http.ListenAndServeTLS(*address, *cert, *key, nil))
	} else {
		log.Printf("TLS certificate and key not provided; using HTTP")
		log.Fatal(http.ListenAndServe(*address, nil))
	}
}
