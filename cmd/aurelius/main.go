package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sb/aurelius/database"

	"github.com/gorilla/mux"
)

func main() {
	var (
		address = flag.String(
			"address", "", "address at which to listen for connections; overrides port setting")
		port     = flag.Int("port", 9090, "port on which to listen for connections")
		cert     = flag.String("cert", "", "TLS certificate file")
		key      = flag.String("key", "", "TLS key file")
		logLevel = flag.Int("log", 1, fmt.Sprintf("log verbosity (0-%v)", database.LogLevelCount))
		dbPath   = flag.String("db", ".", "path to database root")
	)
	flag.Parse()

	if len(*address) == 0 {
		*address = fmt.Sprintf(":%v", *port)
	}

	var assetsDir string
	{
		executable, err := os.Executable()
		if err != nil {
			panic(err)
		}
		assetsDir = filepath.Dir(executable)
	}

	database.SetLogLevel(database.LogLevel(*logLevel - 1))
	db, err := database.New("/db", *dbPath, "html")
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}

	router := mux.NewRouter()
	router.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/db/", http.StatusFound)
	})
	router.PathPrefix(db.Prefix() + "/").Handler(db)
	router.PathPrefix("/static/").Handler(fileOnlyServer{assetsDir})

	http.Handle("/", router)

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
