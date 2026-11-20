package main

import (
	"path/filepath"
	"testing"
)

func TestGenerate(t *testing.T) {
	outdir := t.TempDir()
	cfg := generatorConfig{
		txInterval:   1,
		txCount:      10,
		forkInterval: 2,
		chainLength:  30,
		outputDir:    outdir,
		outputs:      outputFunctionNames(),
	}
	cfg, err := cfg.withDefaults()
	if err != nil {
		t.Fatal(err)
	}
	g := newGenerator(cfg)
	if err := g.run(); err != nil {
		t.Fatal(err)
	}

	names, _ := filepath.Glob(filepath.Join(outdir, "*"))
	t.Log("output files:", names)
}

func TestForkBlocksPetersburg(t *testing.T) {
	cfg := generatorConfig{forkInterval: 3, chainLength: 60}
	forks := cfg.forkBlocks()
	if forks["constantinople"] != 12 || forks["petersburg"] != 12 {
		t.Errorf("constantinople %d, petersburg %d, want 12 for both", forks["constantinople"], forks["petersburg"])
	}
	if forks["istanbul"] != 18 {
		t.Errorf("istanbul %d, want 18", forks["istanbul"])
	}
}
