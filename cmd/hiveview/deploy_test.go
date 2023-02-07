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
	msg, err := b.buildAll()
	if err != nil {
		renderBuildMsg(msg, &output)
		t.Fatal("esbuild errors:\n\n", output.String())
	}
}
