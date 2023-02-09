package main

import (
	"io/fs"
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
	assets, _ := fs.Sub(embeddedAssets, "assets")

	temp := t.TempDir()
	dfs := newDeployFS(assets, true)
	if err := copyFS(temp, dfs); err != nil {
		t.Fatal("copy error:", err)
	}
}

func TestDeployWithoutBundle(t *testing.T) {
	assets, _ := fs.Sub(embeddedAssets, "assets")

	temp := t.TempDir()
	dfs := newDeployFS(assets, false)
	if err := copyFS(temp, dfs); err != nil {
		t.Fatal("copy error:", err)
	}
}
