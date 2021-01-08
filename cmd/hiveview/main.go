// The hiveview command generates hive result listing files for the result viewer.
// It can also serve the viewer and listing via HTTP (with the -server flag).
package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/ethereum/hive/cmd/hiveview/assets"
	"github.com/gorilla/mux"
)

//go:generate go run github.com/mjibson/esc -pkg assets -o assets/assets.go -prefix assets/ assets/

func main() {
	var (
		serve   = flag.Bool("serve", false, "Enables the HTTP server")
		listing = flag.Bool("listing", false, "Generates listing JSON to stdout")
		config  serverConfig
	)
	flag.StringVar(&config.listenAddr, "addr", "0.0.0.0:8080", "HTTP server listen address")
	flag.StringVar(&config.logdir, "logdir", "workspace/logs", "Path to hive simulator log directory")
	flag.BoolVar(&config.useLocalAssets, "local-assets", false, "Serve result view app from file system")
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
	listenAddr     string
	logdir         string
	useLocalAssets bool
}

func runServer(config serverConfig) {
	// Create handlers.
	logHandler := http.FileServer(http.Dir(config.logdir))
	assetHandler := http.FileServer(assets.Dir(config.useLocalAssets, ""))
	listingHandler := serveListing{dir: config.logdir}
	mux := mux.NewRouter()
	mux.Handle("/listing.json", listingHandler).Methods("GET")
	mux.PathPrefix("/results").Handler(http.StripPrefix("/results/", logHandler))
	mux.PathPrefix("/").Handler(assetHandler)

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
