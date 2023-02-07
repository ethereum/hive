package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"path"
	"strings"
	"sync"
	"time"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/liamg/memoryfs"
)

// bundler creates JS/CSS bundles and caches them in memory.
type bundler struct {
	fsys    fs.FS
	options *esbuild.BuildOptions
	mu      sync.Mutex
	files   map[string]*bundleFile
	mem     *memoryfs.FS
}

type bundleFile struct {
	inputPath  string
	hash       [32]byte
	inputFiles []string
	buildTime  time.Time
	buildMsg   []esbuild.Message
}

func newBundler(fsys fs.FS) *bundler {
	mem := memoryfs.New()
	mem.MkdirAll("bundle", 0755)
	return &bundler{
		mem:   mem,
		fsys:  fsys,
		files: make(map[string]*bundleFile),
	}
}

// add adds a file target.
func (b *bundler) add(name string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	name = path.Clean(strings.TrimPrefix(name, "/"))
	if _, ok := b.files[name]; !ok {
		b.files[name] = &bundleFile{inputPath: name}
	}
}

// fs returns a virtual filesystem containing built bundle files.
// Calling this also ensures all bundles are up-to-date!
func (b *bundler) fs() fs.FS {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.buildAll()
	return b.mem
}

func (b *bundler) buildAll() ([]esbuild.Message, error) {
	var allmsg []esbuild.Message
	var firsterr error
	for _, bf := range b.files {
		msg, err := bf.rebuild(b)
		if err != nil && firsterr == nil {
			firsterr = err
		}
		allmsg = append(allmsg, msg...)
	}
	return allmsg, firsterr
}

// build ensures the given bundle file is built, and returns the bundle.
func (b *bundler) build(name string) (bf *bundleFile, buildmsg []esbuild.Message, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	name = path.Clean(strings.TrimPrefix(name, "/"))
	bf, ok := b.files[name]
	if !ok {
		return nil, nil, fs.ErrNotExist
	}
	buildmsg, err = bf.rebuild(b)
	if err != nil {
		return nil, buildmsg, err
	}
	cpy := *bf
	return &cpy, nil, nil
}

// rebuild builds the bundle if necessary.
func (bf *bundleFile) rebuild(b *bundler) ([]esbuild.Message, error) {
	if !bf.needsBuild(b) {
		return nil, nil
	}
	return bf.build(b)
}

// name returns the output file name (including the hash).
func (bf *bundleFile) name() string {
	base := path.Base(bf.inputPath)
	baseNoExt := strings.TrimSuffix(base, path.Ext(base))
	return fmt.Sprintf("%s.%x%s", baseNoExt, bf.hash[:], path.Ext(base))
}

// needsBuild reports whether the bundle needs to be rebuilt.
func (bf *bundleFile) needsBuild(b *bundler) bool {
	if bf.buildTime.IsZero() {
		return true
	}
	for _, f := range bf.inputFiles {
		stat, err := fs.Stat(b.fsys, f)
		if err != nil || stat.ModTime().After(bf.buildTime) {
			return true
		}
	}
	return false
}

// build creates/updates a bundle.
func (bf *bundleFile) build(b *bundler) ([]esbuild.Message, error) {
	log.Printf("esbuild: %s", bf.inputPath)

	prevName := bf.name()
	startTime := time.Now()

	var loadedFiles []string
	loader := fsLoaderPlugin(b.fsys, &loadedFiles)
	options := esbuild.BuildOptions{
		Bundle:            true,
		LogLevel:          esbuild.LogLevelInfo,
		EntryPoints:       []string{bf.inputPath},
		Plugins:           []esbuild.Plugin{loader},
		Platform:          esbuild.PlatformBrowser,
		Target:            esbuild.ES2020,
		MinifyIdentifiers: true,
		MinifyWhitespace:  true,
		MinifySyntax:      true,
		Alias:             moduleAliases,
	}
	res := esbuild.Build(options)
	msg := append(res.Errors, res.Warnings...)
	if len(res.Errors) > 0 {
		return msg, fmt.Errorf("error in %s", bf.inputPath)
	}
	content := res.OutputFiles[0].Contents
	if len(content) == 0 {
		panic("empty build output")
	}

	// Update the result.
	bf.hash = sha256.Sum256(content)
	bf.buildTime = startTime
	bf.inputFiles = loadedFiles

	// Write output to memfs.
	// fmt.Println("store:", path.Join("bundle", bf.name()))
	err := b.mem.WriteFile(path.Join("bundle", bf.name()), content, 0644)
	if err != nil {
		panic("can't write to memfs: " + err.Error())
	}
	// Ensure the previous output file is gone.
	if prevName != bf.name() {
		b.mem.Remove(path.Join("bundle", prevName))
	}

	return msg, nil
}

// fsLoaderPlugin constructs an esbuild loader plugin that wraps a filesystem.
// The plugin does two things:
//
//   - All file loads are done through the given filesystem.
//   - Loaded paths are appended to the 'loadedFiles' list. Note that it is
//     not safe to access loadedFiles until esbuild.Build has returned.
func fsLoaderPlugin(fsys fs.FS, loadedFiles *[]string) esbuild.Plugin {
	var addedFile = make(map[string]bool)
	var fileListMutex sync.Mutex
	addToLoadedFiles := func(path string) {
		fileListMutex.Lock()
		defer fileListMutex.Unlock()
		if !addedFile[path] {
			*loadedFiles = append(*loadedFiles, path)
			addedFile[path] = true
		}
	}

	return esbuild.Plugin{
		Name: "fsLoader",
		Setup: func(build esbuild.PluginBuild) {
			resOpt := esbuild.OnResolveOptions{Filter: ".*"}
			build.OnResolve(resOpt, func(args esbuild.OnResolveArgs) (esbuild.OnResolveResult, error) {
				var p string
				switch args.Kind {
				case esbuild.ResolveCSSURLToken:
					// url(...) in CSS is always considered an external resource.
					return esbuild.OnResolveResult{Path: args.Path, External: true}, nil

				case esbuild.ResolveEntryPoint:
					// For the initial entry point in the bundle, args.Importer is set
					// to the absolute working directory, which can't be used, so just treat
					// it as a raw path into the FS.
					p = strings.TrimPrefix(args.Path, "/")

				default:
					// All other import paths are resolved relative to the importing
					// file's location, unless the name is defined as an alias.
					alias, ok := build.InitialOptions.Alias[args.Path]
					if ok {
						p = path.Clean(alias)
						// fmt.Println("resolved alias:", args.Path, "=>", p)
					} else {
						p = path.Join(path.Dir(args.Importer), args.Path)
					}
				}

				res := esbuild.OnResolveResult{Path: p, Namespace: "fsLoader"}
				_, err := fs.Stat(fsys, p)
				if errors.Is(err, fs.ErrNotExist) && !strings.HasPrefix(args.Path, ".") {
					err = fmt.Errorf("File %s does not exist. Missing definition in moduleAliases?", p)
				}
				return res, err
			})

			loadOpt := esbuild.OnLoadOptions{Filter: ".*", Namespace: "fsLoader"}
			build.OnLoad(loadOpt, func(args esbuild.OnLoadArgs) (esbuild.OnLoadResult, error) {
				addToLoadedFiles(args.Path)
				text, err := fs.ReadFile(fsys, args.Path)
				if err != nil {
					return esbuild.OnLoadResult{}, err
				}
				str := string(text)
				return esbuild.OnLoadResult{
					Contents:   &str,
					ResolveDir: path.Dir(args.Path),
					Loader:     loaderFromExt(args.Path),
				}, nil
			})
		},
	}
}

func loaderFromExt(name string) esbuild.Loader {
	switch path.Ext(name) {
	case ".css":
		return esbuild.LoaderCSS
	case ".js", ".mjs":
		return esbuild.LoaderJS
	case ".ts":
		return esbuild.LoaderTS
	case ".json":
		return esbuild.LoaderJSON
	default:
		return esbuild.LoaderNone
	}
}

func renderBuildMsg(msgs []esbuild.Message, w io.Writer) {
	for _, msg := range msgs {
		file := strings.Replace(msg.Location.File, "fsLoader:", "assets/", 1)
		fmt.Fprintf(w, "%s:%d   %s\n", file, msg.Location.Line, msg.Text)
		fmt.Fprintln(w, "  |")
		fmt.Fprintln(w, "  |", msg.Location.LineText)
		fmt.Fprintln(w, "  |")
		fmt.Fprintln(w, "")
	}
}
