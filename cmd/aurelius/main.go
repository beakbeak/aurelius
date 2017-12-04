package main

import (
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
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
		reload   = flag.Bool("reload", false, "reload templates on every use")
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

	var assetsDir string
	{
		executable, err := os.Executable()
		if err != nil {
			panic(err)
		}
		assetsDir = filepath.Dir(executable)
	}

	templateGlob := filepath.Join(assetsDir, "templates", "*")

	var templateProxy util.TemplateProxy
	if *reload {
		templateProxy = util.DynamicTemplateProxy{templateGlob}
	} else {
		templateProxy = util.StaticTemplateProxy{template.Must(template.ParseGlob(templateGlob))}
	}

	db, err := database.New("/db", *dbPath, templateProxy)
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
	router.PathPrefix(db.Prefix() + "/").Methods("GET").HandlerFunc(dbHandleRequest)
	router.PathPrefix("/static/").Handler(fileOnlyServer{assetsDir})
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

type fileOnlyServer struct {
	root string
}

func (srv fileOnlyServer) ServeHTTP(
	w http.ResponseWriter,
	req *http.Request,
) {
	path := filepath.Join(srv.root, req.URL.Path)
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		http.ServeFile(w, req, path)
		return
	}
	http.NotFound(w, req)
}
