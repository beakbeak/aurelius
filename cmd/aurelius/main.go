package main

import (
	"flag"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sb/aurelius/pkg/media"
	"strings"

	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/vharitonsky/iniflags"
)

const sessionName = "aurelius"

func main() {
	var (
		address = flag.String(
			"listen", ":9090", "[address][:port] at which to listen for connections.")
		cert       = flag.String("cert", "", "TLS certificate file.")
		key        = flag.String("key", "", "TLS key file.")
		mediaPath  = flag.String("media", ".", "Path to media library root.")
		noThrottle = flag.Bool(
			"noThrottle", false, "Don't limit streaming throughput to playback speed.")
		passphrase = flag.String(
			"pass", "",
			`Passphrase used for login. If unspecified, access will not be restricted.

WARNING: Passphrases from the client will be transmitted as plain text,
so use of HTTPS is recommended.`)
	)

	// Reword usage strings of flags from iniflags package
	if configFlag := flag.Lookup("config"); configFlag != nil {
		configFlag.Usage =
			"Path to ini file containing values for command-line flags in 'flagName = value' format. "
	}
	if dumpflagsFlag := flag.Lookup("dumpflags"); dumpflagsFlag != nil {
		dumpflagsFlag.Usage =
			"Print values for all command-line flags to stdout in a format compatible with -config, then exit."
	}

	iniflags.Parse()

	var assetsDir string
	{
		executable, err := os.Executable()
		if err != nil {
			panic(err)
		}
		assetsDir = filepath.Join(filepath.Dir(executable), "assets")
	}

	htmlPath := func(fileName string) string {
		return filepath.Join(assetsDir, "html", fileName)
	}

	media.SetLogger(log.New(log.Writer(), "", log.LstdFlags))

	mlConfig := media.NewLibraryConfig()
	mlConfig.RootPath = *mediaPath
	mlConfig.ThrottleStreaming = !*noThrottle

	ml, err := media.NewLibrary(mlConfig)
	if err != nil {
		log.Fatalf("failed to open media library: %v", err)
	}

	sessionStore := sessions.NewCookieStore(securecookie.GenerateRandomKey(32))

	isAuthorized := func(req *http.Request) bool {
		if *passphrase == "" {
			return true
		}

		session, err := sessionStore.Get(req, sessionName)
		if err != nil {
			return false
		}
		if valid, ok := session.Values["valid"]; ok {
			if validBool, ok := valid.(bool); ok {
				return validBool
			}
		}
		return false
	}

	loginIfUnauthorized := func(w http.ResponseWriter, req *http.Request) bool {
		if isAuthorized(req) {
			return false
		}
		http.Redirect(w, req, "/login?from="+url.QueryEscape(req.URL.String()), http.StatusFound)
		return true
	}

	trySaveSessionValues := func(w http.ResponseWriter, req *http.Request, values ...interface{}) bool {
		session, _ := sessionStore.Get(req, sessionName)

		for i := 0; i+1 < len(values); i += 2 {
			session.Values[values[i]] = values[i+1]
		}

		if err = session.Save(req, w); err != nil {
			log.Printf("session.Save failed: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return false
		}
		return true
	}

	router := mux.NewRouter()

	router.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if loginIfUnauthorized(w, req) {
			return
		}
		http.Redirect(w, req, mlConfig.Prefix+"/", http.StatusFound)
	})

	router.Path("/login").Methods("GET", "POST").HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if *passphrase == "" {
			http.NotFound(w, req)
			return
		}

		if req.Method == "GET" {
			http.ServeFile(w, req, htmlPath("login.html"))
			return
		}

		if err := req.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if req.PostForm.Get("passphrase") != *passphrase {
			query := url.Values{}
			query.Set("from", req.URL.Query().Get("from"))
			query.Set("failed", "")

			loginUrl := url.URL{Path: "/login", RawQuery: query.Encode()}

			http.Redirect(w, req, loginUrl.String(), http.StatusFound)
			return
		}

		if !trySaveSessionValues(w, req, "valid", true) {
			return
		}

		fromUrl := req.URL.Query().Get("from")
		if fromUrl == "" {
			fromUrl = "/"
		}
		http.Redirect(w, req, fromUrl, http.StatusFound)
	})

	router.Path("/logout").HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		trySaveSessionValues(w, req, "valid", false)
	})

	router.PathPrefix(mlConfig.Prefix + "/").HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if loginIfUnauthorized(w, req) {
			return
		}
		if !ml.ServeHTTP(w, req) {
			http.ServeFile(w, req, htmlPath("main.html"))
		}
	})

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

// A fileOnlyServer serves local files from the directory tree rooted at root.
// Requests for directories are rejected.
type fileOnlyServer struct {
	root string
}

func (srv fileOnlyServer) ServeHTTP(
	w http.ResponseWriter,
	req *http.Request,
) {
	path := filepath.Join(srv.root, req.URL.Path)

	if info, err := os.Stat(path); err != nil || info.IsDir() {
		http.NotFound(w, req)
		return
	}

	if strings.ToLower(filepath.Ext(path)) == ".svgz" {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Content-Encoding", "gzip")
	}

	http.ServeFile(w, req, path)
}
