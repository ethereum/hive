package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	esbuild "github.com/evanw/esbuild/pkg/api"
	"github.com/liamg/memoryfs"
)

// bundler creates JS/CSS bundles and caches them in memory.
type bundler struct {
	fsys fs.FS

	mu              sync.Mutex
	files           map[string]*bundleFile
	buildContext    esbuild.BuildContext
	mem             *memoryfs.FS
	lastBuild       time.Time
	lastBuildFailed bool
}

type bundleFile struct {
	outputFile string
	buildTime  time.Time
}

func newBundler(fsys fs.FS, entrypoints []string, aliases map[string]string) *bundler {
	options := makeBuildOptions(fsys)
	options.Alias = aliases
	options.EntryPoints = entrypoints
	ctx, err := esbuild.Context(options)
	if err != nil {
		panic(err)
	}

	return &bundler{
		mem:          memoryfs.New(),
		fsys:         fsys,
		files:        make(map[string]*bundleFile),
		buildContext: ctx,
	}
}

func makeBuildOptions(fsys fs.FS) esbuild.BuildOptions {
	loader := fsLoaderPlugin(fsys)
	return esbuild.BuildOptions{
		Bundle:            true,
		Outdir:            "/",
		AbsWorkingDir:     "/",
		PublicPath:        "/bundle",
		EntryNames:        "[dir]/[name].[hash]",
		AssetNames:        "assets/[dir]/[name].[hash]",
		ChunkNames:        "chunks/chunk-[hash]",
		Splitting:         true,
		Format:            esbuild.FormatESModule,
		LogLevel:          esbuild.LogLevelWarning,
		Plugins:           []esbuild.Plugin{loader},
		Platform:          esbuild.PlatformBrowser,
		Target:            esbuild.ES2020,
		Metafile:          true,
		MinifyIdentifiers: true,
		MinifyWhitespace:  true,
		MinifySyntax:      true,
	}
}

// bundle looks up a bundle output file.
func (b *bundler) bundle(name string) *bundleFile {
	b.mu.Lock()
	defer b.mu.Unlock()

	name = path.Clean(strings.TrimPrefix(name, "/"))
	return b.files[name]
}

// rebuild builds all input files.
func (b *bundler) rebuild() ([]esbuild.Message, fs.FS, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Skip build if last build was < 1s ago.
	if !b.lastBuild.IsZero() && time.Since(b.lastBuild) < 1*time.Second && !b.lastBuildFailed {
		return nil, b.mem, nil
	}
	b.lastBuild = time.Now()

	start := time.Now()
	var msg []esbuild.Message
	var err error
	res := b.buildContext.Rebuild()
	msg = append(msg, res.Errors...)
	msg = append(msg, res.Warnings...)
	b.lastBuildFailed = len(res.Errors) > 0
	if b.lastBuildFailed {
		err = errors.New("build failed")
	} else {
		b.handleBuildResult(&res)
	}

	fmt.Println("build done:", time.Since(start))
	return msg, b.mem, err
}

type metafile struct {
	Outputs map[string]*metafileOutput
}

type metafileOutput struct {
	EntryPoint string
}

func (b *bundler) handleBuildResult(res *esbuild.BuildResult) {
	var meta metafile
	err := json.Unmarshal([]byte(res.Metafile), &meta)
	if err != nil {
		panic("invalid metafile! " + err.Error())
	}

	// Write output files to a new memfs.
	memfs := memoryfs.New()
	now := time.Now()
	for _, f := range res.OutputFiles {
		outputName := strings.TrimPrefix(filepath.ToSlash(f.Path), "/")
		outputPath := "bundle/" + outputName
		m := meta.Outputs[outputName]
		if m == nil {
			panic("unknown output file: " + f.Path)
		}

		// Carry over modification time from the previous memfs.
		modTime := now
		info, err := b.mem.Stat(outputPath)
		if err == nil {
			modTime = info.ModTime()
		}

		// For output files that correspond to an entry point,
		// assign the bundle file name.
		if m.EntryPoint != "" {
			ep := strings.TrimPrefix(m.EntryPoint, "fsLoader:")
			prev := b.files[ep]
			bf := &bundleFile{outputFile: outputName, buildTime: modTime}
			if prev == nil || prev.outputFile != outputName {
				// File has changed, log it.
				log.Println("esbuild", ep, "=>", outputName)
			}
			b.files[ep] = bf
		}

		// fmt.Println("store:", outputPath)
		memfs.MkdirAll(path.Dir(outputPath), 0755)
		if err := memfs.WriteFile(outputPath, f.Contents, 0644); err != nil {
			panic("can't write to memfs: " + err.Error())
		}
		memfs.SetModified(outputPath, modTime)
	}

	// Flip over to the new memfs.
	b.mem = memfs
}

// fsLoaderPlugin constructs an esbuild loader plugin that wraps a filesystem.
func fsLoaderPlugin(fsys fs.FS) esbuild.Plugin {
	return esbuild.Plugin{
		Name: "fsLoader",
		Setup: func(build esbuild.PluginBuild) {
			resOpt := esbuild.OnResolveOptions{Filter: ".*"}
			build.OnResolve(resOpt, func(args esbuild.OnResolveArgs) (esbuild.OnResolveResult, error) {
				// Ignore data: URLs.
				if strings.HasPrefix(args.Path, "data:") {
					return esbuild.OnResolveResult{Path: args.Path, External: true}, nil
				}

				var p string
				if args.Kind == esbuild.ResolveEntryPoint {
					// For the initial entry point in the bundle, args.Importer is set
					// to the absolute working directory, which can't be used, so just treat
					// it as a raw path into the FS.
					p = strings.TrimPrefix(args.Path, "/")
				} else {
					alias, isAlias := build.InitialOptions.Alias[args.Path]
					if isAlias {
						p = path.Clean(alias)
					} else {
						// Relative import paths are resolved relative to the
						// importing file's location.
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
		return esbuild.LoaderFile
	}
}

func renderBuildMsg(msgs []esbuild.Message, w io.Writer) {
	for _, msg := range msgs {
		if msg.Location == nil {
			fmt.Fprintln(w, msg.Text)
			continue
		}
		file := strings.Replace(msg.Location.File, "fsLoader:", "assets/", 1)
		fmt.Fprintf(w, "%s:%d   %s\n", file, msg.Location.Line, msg.Text)
		fmt.Fprintln(w, "  |")
		fmt.Fprintln(w, "  |", msg.Location.LineText)
		fmt.Fprintln(w, "  |")
		fmt.Fprintln(w, "")
	}
}
