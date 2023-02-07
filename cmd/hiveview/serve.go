package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"
)

//go:embed assets
var embeddedAssets embed.FS

type serverConfig struct {
	listenAddr    string
	logDir        string
	assetsDir     string
	disableBundle bool
}

func (cfg *serverConfig) assetFS() (fs.FS, error) {
	if cfg.assetsDir != "" {
		if stat, _ := os.Stat(cfg.assetsDir); stat == nil || !stat.IsDir() {
			return nil, fmt.Errorf("%q is not a directory", cfg.assetsDir)
		}
		return os.DirFS(cfg.assetsDir), nil
	}
	sub, err := fs.Sub(embeddedAssets, "assets")
	if err != nil {
		panic(err)
	}
	return sub, nil
}

func runServer(config serverConfig) {
	assetFS, err := config.assetFS()
	if err != nil {
		log.Fatalf("-assets: %v", err)
	}

	// Create handlers.
	deployFS := newDeployFS(assetFS, !config.disableBundle)
	logDirFS := os.DirFS(config.logDir)
	logHandler := http.FileServer(http.FS(logDirFS))
	listingHandler := serveListing{fsys: logDirFS}

	mux := mux.NewRouter()
	mux.Handle("/listing.jsonl", listingHandler).Methods("GET")
	mux.PathPrefix("/results").Handler(http.StripPrefix("/results/", logHandler))
	mux.PathPrefix("/").Handler(serveFiles{deployFS})

	// Start the server.
	l, err := net.Listen("tcp", config.listenAddr)
	if err != nil {
		log.Fatalf("Can't listen: %v", err)
	}
	log.Printf("Serving at http://%v/", l.Addr())
	http.Serve(l, mux)
}

type serveListing struct{ fsys fs.FS }

func (h serveListing) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("Generating listing...")
	err := generateListing(h.fsys, ".", w)
	if err != nil {
		fmt.Println("error:", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

type serveFiles struct{ fsys fs.FS }

func (h serveFiles) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Add caching-related headers.
	path := r.URL.Path
	if path == "/" || strings.HasSuffix(path, ".html") {
		w.Header().Set("cache-control", "no-cache")
	}

	srv := http.FileServer(http.FS(h.fsys))
	srv.ServeHTTP(w, r)

}
