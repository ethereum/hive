package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

//go:embed assets
var embeddedAssets embed.FS

type serverConfig struct {
	listenAddr string
	logDir     string
	assetsDir  string
}

func runServer(config serverConfig) {
	var assetFS fs.FS
	if config.assetsDir != "" {
		if stat, _ := os.Stat(config.assetsDir); stat == nil || !stat.IsDir() {
			log.Fatalf("-assets: %q is not a directory", config.assetsDir)
		}
		assetFS = os.DirFS(config.assetsDir)
	} else {
		sub, err := fs.Sub(embeddedAssets, "assets")
		if err != nil {
			panic(err)
		}
		assetFS = sub
	}

	// Create handlers.
	logDirFS := os.DirFS(config.logDir)
	logHandler := http.FileServer(http.FS(logDirFS))
	listingHandler := serveListing{fsys: logDirFS}
	mux := mux.NewRouter()
	mux.Handle("/listing.jsonl", listingHandler).Methods("GET")
	mux.PathPrefix("/results").Handler(http.StripPrefix("/results/", logHandler))
	mux.PathPrefix("/").Handler(http.FileServer(http.FS(assetFS)))

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
