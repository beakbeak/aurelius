package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sb/aurelius/pkg/media"

	"github.com/gorilla/mux"
)

func main() {
	var (
		address = flag.String(
			"listen", ":9090", "[address][:port] at which to listen for connections")
		cert      = flag.String("cert", "", "TLS certificate file")
		key       = flag.String("key", "", "TLS key file")
		logLevel  = flag.Int("log", 1, fmt.Sprintf("log verbosity (0-%v)", media.LogLevelCount))
		mediaPath = flag.String("media", ".", "path to media library root")
	)
	flag.Parse()

	var assetsDir string
	{
		executable, err := os.Executable()
		if err != nil {
			panic(err)
		}
		assetsDir = filepath.Dir(executable)
	}

	media.SetLogLevel(media.LogLevel(*logLevel - 1))

	mlConfig := media.NewLibraryConfig()
	mlConfig.RootPath = *mediaPath
	mlConfig.HtmlPath = "html"

	ml, err := media.NewLibrary(mlConfig)
	if err != nil {
		log.Fatalf("failed to open media library: %v", err)
	}

	router := mux.NewRouter()
	router.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/media/", http.StatusFound)
	})
	router.PathPrefix(mlConfig.Prefix + "/").Handler(ml)
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
