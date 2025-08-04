package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildAllBundles(t *testing.T) {
	assets, _ := fs.Sub(embeddedAssets, "assets")
	b := hiveviewBundler(assets)

	var output strings.Builder
	msg, _, err := b.rebuild()
	if err != nil {
		renderBuildMsg(msg, &output)
		t.Fatal("esbuild errors:\n\n", output.String())
	}
}

func TestDeployWithBundle(t *testing.T) {
	var config serverConfig
	assets, _ := config.assetFS()

	temp := t.TempDir()
	dfs := newDeployFS(assets, &config)
	if err := copyFS(temp, dfs); err != nil {
		t.Fatal("copy error:", err)
	}
	entries, _ := os.ReadDir(filepath.Join(temp, "bundle"))
	if len(entries) == 0 {
		t.Fatal("bundle/ output directory is empty")
	}
}

func TestDeployWithoutBundle(t *testing.T) {
	var config serverConfig
	assets, _ := config.assetFS()
	config.disableBundle = true

	temp := t.TempDir()
	dfs := newDeployFS(assets, &config)
	if err := copyFS(temp, dfs); err != nil {
		t.Fatal("copy error:", err)
	}
	_, staterr := os.Stat(filepath.Join(temp, "bundle"))
	if !errors.Is(staterr, fs.ErrNotExist) {
		t.Fatal("bundle/ should not exist in output directory")
	}
}
