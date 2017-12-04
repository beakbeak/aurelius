package main

// TODO:
// - basic playlist management
// - stream mp3/ogg/flac without re-encoding (must apply ReplayGain on client side)
//   - have to throttle using a fixed bit rate
// - embedded images
// - seeking
// - combine artists/etc. with different capitalizations/whitespace/accents?/brackets/etc.
// - tag overrides in a text file? (necessary for streaming audio track of a video file)
// - DB WAV output

// WISHLIST:
// - replaygain preamp?
// - treat sections of a file as playlist entries (e.g. pieces of a long live set, a hidden track)
//   - can't use m3u anymore
// - tag editing
// - get replaygain info from RVA2 mp3 tag (requires another library dependency)
// - figure out why timing is wrong when using MKV container
//   (maybe something to do with time base settings/conversion)

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"sb/aurelius/aurelib"
	"sb/aurelius/database"
	"sb/aurelius/util"

	"github.com/gorilla/mux"
)

func main() {
	var (
		address = flag.String(
			"address", "", "address at which to listen for connections; overrides port setting")
		port     = flag.Int("port", 9090, "port on which to listen for connections")
		cert     = flag.String("cert", "", "TLS certificate file")
		key      = flag.String("key", "", "TLS key file")
		logLevel = flag.Int("log", 2, "log verbosity (1-3)")
		dbPath   = flag.String("db", ".", "path to database root")
	)
	flag.Parse()

	util.SetLogLevel(*logLevel)
	if *logLevel > 1 {
		aurelib.SetLogLevel(aurelib.LogInfo)
	}

	if len(*address) == 0 {
		*address = fmt.Sprintf(":%v", *port)
	}

	aurelib.NetworkInit()
	defer aurelib.NetworkDeinit()

	db, err := database.New("/db", *dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	dbHandleRequest := func(w http.ResponseWriter, req *http.Request) {
		db.HandleRequest(w, req)
	}

	/*
		player := player.New()
		defer player.Destroy()

		playerHandleRpc := func(w http.ResponseWriter, req *http.Request) {
			player.HandleRpc(w, req)
		}
	*/

	router := mux.NewRouter()
	router.PathPrefix(db.Prefix()).Methods("GET").HandlerFunc(dbHandleRequest)
	/*
		router.HandleFunc("/rpc", playerHandleRpc).
			Methods("POST").
			Headers("Content-Type", "application/json")
	*/

	http.Handle("/", router)

	//player.Play(playerPkg.NewFilePlaylist(flag.Args()))

	log.Printf("listening on %s\n", *address)
	if len(*cert) > 0 || len(*key) > 0 {
		log.Printf("using HTTPS")
		log.Fatal(http.ListenAndServeTLS(*address, *cert, *key, nil))
	} else {
		log.Printf("TLS certificate and key not provided; using HTTP")
		log.Fatal(http.ListenAndServe(*address, nil))
	}
}
