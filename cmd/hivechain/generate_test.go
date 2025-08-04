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
