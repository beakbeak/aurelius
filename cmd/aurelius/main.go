package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/beakbeak/aurelius/internal/media"

	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/vharitonsky/iniflags"
)

const sessionName = "aurelius"
const maxClientLogSize = 1024

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

	level, ok := parseLogLevel(*logLevel)
	if !ok {
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
	defer ml.Close()

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

	loginIfNoAuth := func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if !isAuthorized(req) {
				http.Redirect(w, req, "/login?from="+url.QueryEscape(req.URL.String()), http.StatusFound)
				return
			}
			handler.ServeHTTP(w, req)
		})
	}

	failIfNoAuth := func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if !isAuthorized(req) {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			handler.ServeHTTP(w, req)
		})
	}

	rootHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, mlConfig.Prefix+"/tree/", http.StatusFound)
	})

	loginGetHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if *passphrase == "" || isAuthorized(req) {
			redirectLogin(w, req)
			return
		}
		serveStaticFile(w, req, htmlPath("login.html"))
	})

	loginPostHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if *passphrase == "" {
			http.NotFound(w, req)
			return
		}
		if err := req.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if req.PostForm.Get("passphrase") != *passphrase {
			slog.InfoContext(req.Context(), "login attempt failed")

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
		slog.InfoContext(req.Context(), "login succeeded")
		redirectLogin(w, req)
	})

	logoutHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		trySaveSessionValues(w, req, "valid", false)
	})

	mainPageHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		serveStaticFile(w, req, htmlPath("main.html"))
	})

	mediaHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ml.ServeHTTP(w, req)
	})

	clientLogHandler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req.Body = http.MaxBytesReader(w, req.Body, maxClientLogSize)
		defer req.Body.Close()

		body, err := io.ReadAll(req.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var rawData map[string]interface{}
		if err := json.Unmarshal(body, &rawData); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		msg, ok := rawData["msg"].(string)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		levelStr, ok := rawData["level"].(string)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		level, ok := parseLogLevel(levelStr)
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		attrs := make([]slog.Attr, 0, len(rawData))
		for k, v := range rawData {
			if k != "msg" && k != "level" {
				attrs = append(attrs, slog.Any(k, v))
			}
		}
		slog.LogAttrs(req.Context(), level, "client log: "+msg, attrs...)

		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte("{}")); err != nil {
			slog.ErrorContext(req.Context(), "failed to write response", "error", err)
		}
	})

	router := http.NewServeMux()
	router.Handle("GET /static/", fileOnlyServer{assetsDir})
	router.Handle("GET /login", loginGetHandler)
	router.Handle("POST /login", loginPostHandler)
	router.Handle("GET /logout", logoutHandler)
	router.Handle("POST /logout", logoutHandler)
	router.Handle("GET /", loginIfNoAuth(rootHandler))
	router.Handle("GET "+mlConfig.Prefix+"/tree/", loginIfNoAuth(mainPageHandler))
	router.Handle("GET "+mlConfig.Prefix+"/", failIfNoAuth(withLog(mediaHandler)))
	router.Handle("POST "+mlConfig.Prefix+"/", failIfNoAuth(withLog(mediaHandler)))
	router.Handle("POST /log", failIfNoAuth(clientLogHandler))

	srv := &http.Server{
		Addr:    *listen,
		Handler: withRequestIDAndAddress(router),
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		slog.Info("received signal, shutting down", "signal", sig)
		if err := srv.Shutdown(context.Background()); err != nil {
			slog.Error("server shutdown error", "error", err)
		}
	}()

	log.Printf("listening on %s\n", *listen)
	if len(*tlsCert) > 0 || len(*tlsKey) > 0 {
		log.Printf("using HTTPS")
		if err := srv.ListenAndServeTLS(*tlsCert, *tlsKey); err != http.ErrServerClosed {
			log.Fatal(err)
		}
	} else {
		log.Printf("TLS certificate and key not provided; using HTTP")
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}
}

type contextKey int

const (
	requestIDKey contextKey = iota
	remoteAddressKey
)

// contextLogHandler is a custom slog handler that includes request ID from context.
type contextLogHandler struct {
	slog.Handler
}

func (h *contextLogHandler) Handle(ctx context.Context, r slog.Record) error {
	if requestID, ok := ctx.Value(requestIDKey).(string); ok {
		r.Add("reqID", requestID)
	}
	if remoteAddr, ok := ctx.Value(remoteAddressKey).(string); ok {
		r.Add("addr", remoteAddr)
	}
	return h.Handler.Handle(ctx, r)
}

func withRequestIDAndAddress(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := makeRequestID()
		remoteAddr := r.Header.Get("x-forwarded-for")
		if remoteAddr == "" {
			remoteAddr = r.RemoteAddr
		}
		ctx := r.Context()
		ctx = context.WithValue(ctx, requestIDKey, requestID)
		ctx = context.WithValue(ctx, remoteAddressKey, remoteAddr)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func withLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.InfoContext(r.Context(), "request",
			"method", r.Method,
			"path", r.URL.Path,
			"query", r.URL.RawQuery,
		)
		next.ServeHTTP(w, r)
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

func parseLogLevel(levelStr string) (slog.Level, bool) {
	switch levelStr {
	case "debug":
		return slog.LevelDebug, true
	case "info":
		return slog.LevelInfo, true
	case "warn":
		return slog.LevelWarn, true
	case "error":
		return slog.LevelError, true
	default:
		return slog.LevelInfo, false
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
	serveStaticFile(w, req, filepath.Join(srv.root, req.URL.Path))
}

// serveStaticFile serves a file with ETag-based caching. Directory requests
// are rejected. If a pre-compressed .gz variant exists and the client accepts
// gzip, it is served instead.
func serveStaticFile(w http.ResponseWriter, req *http.Request, path string) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		http.NotFound(w, req)
		return
	}

	servePath := path

	if strings.ToLower(filepath.Ext(path)) == ".svgz" {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Content-Encoding", "gzip")
	} else if strings.Contains(req.Header.Get("Accept-Encoding"), "gzip") {
		gzPath := path + ".gz"
		if gzInfo, err := os.Stat(gzPath); err == nil && !gzInfo.IsDir() {
			servePath = gzPath
			info = gzInfo
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(path)))
			w.Header().Set("Vary", "Accept-Encoding")
		}
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

	http.ServeFile(w, req, servePath)
}
