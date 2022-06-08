// The hiveview command generates hive result listing files for the result viewer.
// It can also serve the viewer and listing via HTTP (with the -server flag).
package main

import (
	"flag"
	"log"
	"os"
	"time"
)

const (
	durationDays  = 24 * time.Hour
	durationMonth = 31 * durationDays
)

func main() {
	var (
		serve          = flag.Bool("serve", false, "Enables the HTTP server")
		listing        = flag.Bool("listing", false, "Generates listing JSON to stdout")
		gc             = flag.Bool("gc", false, "Deletes old log files")
		gcKeepInterval = flag.Duration("keep", 5*durationMonth, "Time interval of past log files to keep")
		config         serverConfig
	)
	flag.StringVar(&config.listenAddr, "addr", "0.0.0.0:8080", "HTTP server listen address")
	flag.StringVar(&config.logDir, "logdir", "workspace/logs", "Path to hive simulator log directory")
	flag.StringVar(&config.assetsDir, "assets", "", "Path to static files directory. Serves baked-in assets when not set.")
	flag.Parse()

	log.SetFlags(log.LstdFlags)
	switch {
	case *serve:
		runServer(config)
	case *listing:
		fsys := os.DirFS(config.logDir)
		generateListing(fsys, ".", os.Stdout)
	case *gc:
		cutoff := time.Now().Add(-*gcKeepInterval)
		logdirGC(config.logDir, cutoff)
	default:
		log.Fatalf("Use -serve or -listing to select mode")
	}
}
