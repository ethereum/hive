package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// hiveviewBundler creates the esbuild bundler and registers JS/CSS targets.
func hiveviewBundler(fsys fs.FS) *bundler {
	entrypoints := []string{
		"lib/app-index.js",
		"lib/app-suite.js",
		"lib/app-viewer.js",
		"lib/app.css",
		"lib/viewer.css",
	}
	b := newBundler(fsys, entrypoints, moduleAliases)
	return b
}

// moduleAliases maps ES module names to files.
var moduleAliases = map[string]string{
	"jquery":                        "./extlib/jquery-3.6.3.esm.js",
	"@popper/core":                  "./extlib/popper-2.9.2.js",
	"bootstrap":                     "./extlib/bootstrap-5.2.3.mjs",
	"datatables.net":                "./extlib/dataTables-1.13.1.mjs",
	"datatables.net-bs5":            "./extlib/dataTables-1.13.1.bootstrap5.mjs",
	"datatables.net-responsive":     "./extlib/responsive-2.4.0.mjs",
	"datatables.net-responsive-bs5": "./extlib/responsive-2.4.0.bootstrap5.mjs",
}

func importMapScript() string {
	im := map[string]any{"imports": moduleAliases}
	imdata, _ := json.Marshal(im)
	return `<script type="importmap">` + string(imdata) + `</script>`
}

// deployFS is a virtual overlay file system for the assets directory.
// It mostly acts as a pass-through for the 'assets' file system, except for
// two special cases:
//
//   - The overlay adds a bundle/ directory containing built JS/CSS bundles.
//   - For .html files in the root directory, all JS/CSS references are checked
//     against the bundler, and URLs in the HTML will be replaced by references to
//     bundle files.
type deployFS struct {
	assets  fs.FS
	bundler *bundler
}

func newDeployFS(assets fs.FS, useBundle bool) *deployFS {
	dfs := &deployFS{assets: assets}
	if useBundle {
		dfs.bundler = hiveviewBundler(assets)
	}
	return dfs
}

func isBundlePath(name string) bool {
	return name == "bundle" || strings.HasPrefix(name, "bundle/")
}

var _ fs.FS = (*deployFS)(nil)

// Open opens a file.
func (dfs *deployFS) Open(name string) (f fs.File, err error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "Open", Path: name, Err: fs.ErrInvalid}
	}
	switch {
	case !strings.Contains(name, "/") && strings.HasSuffix(name, ".html"):
		return dfs.openHTML(name)
	case dfs.bundler != nil && isBundlePath(name):
		_, memfs, _ := dfs.bundler.rebuild()
		return memfs.Open(name)
	default:
		return dfs.assets.Open(name)
	}
}

var _ fs.ReadDirFS = (*deployFS)(nil)

// ReadDir reads a directory.
func (dfs *deployFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "ReadDir", Path: name, Err: fs.ErrInvalid}
	}
	switch {
	case name == ".":
		return dfs.readDirRoot()
	case dfs.bundler != nil && isBundlePath(name):
		_, memfs, _ := dfs.bundler.rebuild()
		return fs.ReadDir(memfs, name)
	default:
		return fs.ReadDir(dfs.assets, name)
	}
}

func (dfs *deployFS) readDirRoot() ([]fs.DirEntry, error) {
	entries, err := fs.ReadDir(dfs.assets, ".")
	if err != nil {
		return nil, err
	}
	if dfs.bundler != nil {
		_, memfs, _ := dfs.bundler.rebuild()
		bundleEntries, _ := fs.ReadDir(memfs, ".")
		entries = append(entries, bundleEntries...)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	return entries, nil
}

// openHTML opens a HTML file in the root directory and modifies it to use
// bundled JS/CSS files.
func (dfs *deployFS) openHTML(name string) (fs.File, error) {
	inputFile, err := dfs.assets.Open(name)
	if err != nil {
		return nil, err
	}
	defer inputFile.Close()

	inputInfo, err := inputFile.Stat()
	if err != nil {
		return nil, err
	}
	output := new(bytes.Buffer)
	modTime := inputInfo.ModTime()

	if dfs.bundler == nil {
		// JS bundle is disabled. To make ES module loading work without the bundle,
		// the document needs an importmap.
		insertAfterTag(inputFile, output, "head", importMapScript())
		modTime = time.Now()
	} else {
		// Replace script/style references with bundle paths, if possible.
		buildmsg, _, _ := dfs.bundler.rebuild()
		var errorShown bool
		modifyHTML(inputFile, output, func(token *html.Token, errlog io.Writer) {
			if len(buildmsg) > 0 && !errorShown {
				io.WriteString(errlog, "** ESBUILD ERRORS **\n\n")
				renderBuildMsg(buildmsg, errlog)
				modTime = time.Now()
				errorShown = true
			}

			ref := scriptOrStyleReference(token)
			if ref == nil {
				return // not script
			}
			bundle := dfs.bundler.bundle(ref.Val)
			if bundle == nil || bundle.outputFile == "" {
				return // not a bundle target
			}
			if bundle.buildTime.After(modTime) {
				modTime = bundle.buildTime
			}
			ref.Val = bundle.outputFile
		})
	}

	content := output.Bytes()
	file := newMemFile(inputInfo.Name(), modTime, content)
	return file, nil
}

// insertAfterTag adds content to the document after the first occurrence of a HTML tag.
// The resulting document is written to w.
func insertAfterTag(r io.Reader, w io.Writer, tagName, content string) error {
	var done bool
	z := html.NewTokenizer(r)
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			if z.Err() == io.EOF {
				return nil
			}
			return z.Err()
		} else {
			w.Write(z.Raw())
			if !done && tt == html.StartTagToken {
				name, _ := z.TagName()
				if string(name) == tagName {
					io.WriteString(w, content)
					done = true
				}
			}
		}
	}
}

// modifyScripts changes the 'src' URL of all script tags using the given function.
func modifyHTML(r io.Reader, w io.Writer, modify func(tag *html.Token, errlog io.Writer)) error {
	var errlog bytes.Buffer
	z := html.NewTokenizer(r)
	for {
		tt := z.Next()
		switch tt {
		case html.ErrorToken:
			if z.Err() == io.EOF {
				return nil
			}
			return z.Err()
		case html.StartTagToken, html.SelfClosingTagToken:
			token := z.Token()
			modify(&token, &errlog)
			io.WriteString(w, token.String())
		case html.EndTagToken:
			// Insert the build error log at end of body.
			tag, _ := z.TagName()
			if string(tag) == "body" && errlog.Len() > 0 {
				logHTML := `<pre style="position: absolute; top: 20px; left: 20px; width: 90%; border: 4px solid black; background-color: white; padding: 1em;">`
				logHTML += html.EscapeString(errlog.String())
				logHTML += `</pre>`
				io.WriteString(w, logHTML)
			}
			w.Write(z.Raw())
		default:
			w.Write(z.Raw())
		}
	}
}

func scriptOrStyleReference(token *html.Token) *html.Attribute {
	switch token.Data {
	case "script":
		return findAttr(token, "src")
	case "link":
		rel := findAttr(token, "rel")
		if rel != nil && rel.Val == "stylesheet" {
			return findAttr(token, "href")
		}
	}
	return nil
}

func findAttr(token *html.Token, key string) *html.Attribute {
	for i, a := range token.Attr {
		if a.Namespace == "" && a.Key == key {
			return &token.Attr[i]
		}
	}
	return nil
}

// memFile is a virtual fs.File backed by []byte.
type memFile struct {
	*bytes.Reader
	modTime time.Time
	name    string
}

func newMemFile(name string, modTime time.Time, content []byte) *memFile {
	return &memFile{bytes.NewReader(content), modTime, name}
}

func (f *memFile) Close() error               { return nil }
func (f *memFile) Stat() (fs.FileInfo, error) { return f, nil }
func (f *memFile) Name() string               { return f.name }
func (f *memFile) Mode() fs.FileMode          { return 0644 }
func (f *memFile) ModTime() time.Time         { return f.modTime }
func (f *memFile) IsDir() bool                { return false }
func (f *memFile) Sys() any                   { return nil }
