// The hiveview command generates hive result listing files for the result viewer.
// It can also serve the viewer and listing via HTTP (with the -server flag).
package main

import (
	"embed"
	"flag"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

//go:embed assets
var embeddedAssets embed.FS

func main() {
	var (
		serve   = flag.Bool("serve", false, "Enables the HTTP server")
		listing = flag.Bool("listing", false, "Generates listing JSON to stdout")
		config  serverConfig
	)
	flag.StringVar(&config.listenAddr, "addr", "0.0.0.0:8080", "HTTP server listen address")
	flag.StringVar(&config.logdir, "logdir", "workspace/logs", "Path to hive simulator log directory")
	flag.StringVar(&config.assetsDir, "assets", "", "Path to static files directory. Serves baked-in assets when not set.")
	flag.Parse()

	log.SetFlags(log.LstdFlags)
	switch {
	case *serve:
		runServer(config)
	case *listing:
		generateListing(os.Stdout, config.logdir)
	default:
		log.Fatalf("Use -serve or -listing to select mode")
	}
}

type serverConfig struct {
	listenAddr string
	logdir     string
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
	logHandler := http.FileServer(http.Dir(config.logdir))
	listingHandler := serveListing{dir: config.logdir}
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

type serveListing struct{ dir string }

func (h serveListing) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("Generating listing...")
	err := generateListing(w, h.dir)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
