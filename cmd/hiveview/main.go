// The hiveview command generates hive result listing files for the result viewer.
// It can also serve the viewer and listing via HTTP (with the -server flag).
package main

import (
	"flag"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
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
		deploy         = flag.Bool("deploy", false, "Compiles the frontend to a static directory")
		gc             = flag.Bool("gc", false, "Deletes old log files")
		gcKeepInterval = flag.Duration("keep", 5*durationMonth, "Time interval of past log files to keep (for -gc)")
		gcKeepMin      = flag.Int("keep-min", 10, "Minmum number of suite outputs to keep (for -gc)")
		config         serverConfig
	)
	flag.StringVar(&config.listenAddr, "addr", "0.0.0.0:8080", "HTTP server listen address")
	flag.StringVar(&config.logDir, "logdir", "workspace/logs", "Path to hive simulator log directory")
	flag.StringVar(&config.assetsDir, "assets", "", "Path to static files directory. Serves baked-in assets when not set.")
	flag.BoolVar(&config.disableBundle, "assets.nobundle", false, "Disables JS/CSS bundling (for development).")
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
		logdirGC(config.logDir, cutoff, *gcKeepMin)
	case *deploy:
		doDeploy(&config)
	default:
		log.Fatalf("Use -serve or -listing to select mode")
	}
}

// doDeploy writes the UI to a directory.
func doDeploy(config *serverConfig) {
	if flag.NArg() != 1 {
		log.Fatalf("-deploy requires output directory as argument")
	}
	outputDir := flag.Arg(0)
	assetFS, err := config.assetFS()
	if err != nil {
		log.Fatalf("-assets: %v", err)
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatal(err)
	}

	deploy := newDeployFS(assetFS, !config.disableBundle)
	if err := copyFS(outputDir, deploy); err != nil {
		log.Fatal(err)
	}
}

// copyFS walks the specified root directory on src and copies directories and
// files to dest filesystem.
func copyFS(dest string, src fs.FS) error {
	return fs.WalkDir(src, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d == nil {
			return err
		}
		destPath := filepath.Join(dest, filepath.FromSlash(path))
		if d.IsDir() {
			return os.MkdirAll(destPath, 0755)
		}
		srcFile, err := src.Open(path)
		if err != nil {
			return err
		}
		destFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}
		defer destFile.Close()
		log.Println("copy", path)
		_, err = io.Copy(destFile, srcFile)
		return err
	})
}
