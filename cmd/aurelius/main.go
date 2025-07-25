package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/beakbeak/aurelius/pkg/media"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/vharitonsky/iniflags"
)

const sessionName = "aurelius"

func main() {
	var (
		listen = flag.String(
			"listen", ":9090", "[address][:port] at which to listen for connections.")
		tlsCert     = flag.String("cert", "", "TLS certificate file.")
		tlsKey      = flag.String("key", "", "TLS key file.")
		mediaPath   = flag.String("media", ".", "Path to media library root.")
		storagePath = flag.String(
			"storage", ".",
			`Path to directory where persistent data (favorites, etc.) will be stored.
It will be created if it doesn't exist.`)
		noThrottle = flag.Bool(
			"noThrottle", false, "Don't limit streaming throughput to playback speed.")
		passphrase = flag.String(
			"pass", "",
			`Passphrase used for login. If unspecified, access will not be restricted.

WARNING: Passphrases from the client will be transmitted as plain text,
so use of HTTPS is recommended.`)
		logLevel = flag.String(
			"log", "info", "Log level: debug, info, warn, error.")
	)

	// Reword usage strings of flags from iniflags package
	if configFlag := flag.Lookup("config"); configFlag != nil {
		configFlag.Usage =
			"Path to ini file containing values for command-line flags in 'flagName = value' format."
	}
	if dumpflagsFlag := flag.Lookup("dumpflags"); dumpflagsFlag != nil {
		dumpflagsFlag.Usage =
			"Print values for all command-line flags to stdout in a format compatible with -config, then exit."
	}

	iniflags.Parse()

	var level slog.Level
	switch strings.ToLower(*logLevel) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		log.Fatalf("invalid log level: %s", *logLevel)
	}

	logHandler := &contextLogHandler{Handler: slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})}
	slog.SetDefault(slog.New(logHandler))

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

	mlConfig := media.NewLibraryConfig()
	mlConfig.RootPath = *mediaPath
	mlConfig.StoragePath = *storagePath
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

	requireAuth := func(handler http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, req *http.Request) {
			if loginIfUnauthorized(w, req) {
				return
			}
			handler(w, req)
		}
	}

	rootHandler := func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, mlConfig.Prefix+"/tree/", http.StatusFound)
	}

	loginGetHandler := func(w http.ResponseWriter, req *http.Request) {
		if *passphrase == "" || isAuthorized(req) {
			redirectLogin(w, req)
			return
		}
		http.ServeFile(w, req, htmlPath("login.html"))
	}

	loginPostHandler := func(w http.ResponseWriter, req *http.Request) {
		if *passphrase == "" {
			http.NotFound(w, req)
			return
		}

		if err := req.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if req.PostForm.Get("passphrase") != *passphrase {
			slog.Info("login attempt failed", "remote_addr", req.RemoteAddr)

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

		slog.Info("login succeeded", "remote_addr", req.RemoteAddr)
		redirectLogin(w, req)
	}

	logoutHandler := func(w http.ResponseWriter, req *http.Request) {
		trySaveSessionValues(w, req, "valid", false)
	}

	mainPageHandler := func(w http.ResponseWriter, req *http.Request) {
		http.ServeFile(w, req, htmlPath("main.html"))
	}

	mediaHandler := func(w http.ResponseWriter, req *http.Request) {
		ml.ServeHTTP(w, req)
	}

	router := http.NewServeMux()
	router.Handle("GET /static/", fileOnlyServer{assetsDir})
	router.HandleFunc("GET /login", loginGetHandler)
	router.HandleFunc("POST /login", loginPostHandler)
	router.HandleFunc("GET /logout", logoutHandler)
	router.HandleFunc("POST /logout", logoutHandler)
	router.HandleFunc("GET /", requireAuth(rootHandler))
	router.HandleFunc("GET "+mlConfig.Prefix+"/tree/", requireAuth(mainPageHandler))
	router.HandleFunc("GET "+mlConfig.Prefix+"/", requireAuth(mediaHandler))
	router.HandleFunc("POST "+mlConfig.Prefix+"/", requireAuth(mediaHandler))

	http.Handle("/", withRequestID(router))

	log.Printf("listening on %s\n", *listen)
	if len(*tlsCert) > 0 || len(*tlsKey) > 0 {
		log.Printf("using HTTPS")
		log.Fatal(http.ListenAndServeTLS(*listen, *tlsCert, *tlsKey, nil))
	} else {
		log.Printf("TLS certificate and key not provided; using HTTP")
		log.Fatal(http.ListenAndServe(*listen, nil))
	}
}

type contextKey int

const requestIDKey contextKey = 0

// contextLogHandler is a custom slog handler that includes request ID from context.
type contextLogHandler struct {
	slog.Handler
}

func (h *contextLogHandler) Handle(ctx context.Context, r slog.Record) error {
	if requestID, ok := ctx.Value(requestIDKey).(string); ok {
		r.Add("reqID", requestID)
	}
	return h.Handler.Handle(ctx, r)
}

// withRequestID is middleware that adds a 64-bit ID to the request context and logs request details.
func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := makeRequestID()
		remoteAddr := r.Header.Get("x-forwarded-for")
		if remoteAddr == "" {
			remoteAddr = r.RemoteAddr
		}
		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		slog.InfoContext(ctx, "request",
			"method", r.Method,
			"path", r.URL.Path,
			"query", r.URL.RawQuery,
			"remote_addr", remoteAddr,
		)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// makeRequestID creates a 64-bit random ID as a hex string.
func makeRequestID() string {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic(fmt.Sprintf("failed to generate request ID: %v", err))
	}
	return fmt.Sprintf("%016x", binary.BigEndian.Uint64(buf[:]))
}

func redirectLogin(w http.ResponseWriter, req *http.Request) {
	fromUrl := req.URL.Query().Get("from")
	if fromUrl == "" {
		fromUrl = "/"
	}
	http.Redirect(w, req, fromUrl, http.StatusFound)
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

	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		http.NotFound(w, req)
		return
	}

	etag := fmt.Sprintf("\"%x-%x\"", info.ModTime().Unix(), info.Size())
	w.Header().Set("ETag", etag)
	if match := req.Header.Get("If-None-Match"); match != "" {
		if match == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	w.Header().Set("Cache-Control", "no-cache")

	if strings.ToLower(filepath.Ext(path)) == ".svgz" {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Content-Encoding", "gzip")
	}

	http.ServeFile(w, req, path)
}
