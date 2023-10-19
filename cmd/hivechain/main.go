// The hivechain command assists with generating blockchain data for testing purposes.
//
// The 'generate' subcommand mines a new chain:
//
//	hivechain generate -length 10 -genesis ./genesis.json -blocktime 30 -output .
//
// The 'print' subcommand displays blocks in a chain.rlp file:
//
//	hivechain print -v chain.rlp
//
// The 'print-genesis' subcommand displays the block header fields of a genesis.json file:
//
//	hivechain print-genesis genesis.json
//
// The 'trim' subcommand extracts a range of blocks from a chain.rlp file:
//
//	hivechain trim -from 10 -to 100 chain.rlp newchain.rlp
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/core/types"
	ethlog "github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
)

const usage = "Usage: hivechain generate|print|print-genesis|trim [ options ] ..."

func main() {
	// Initialize go-ethereum logging.
	// This is mostly for displaying the DAG generator progress.
	handler := ethlog.StreamHandler(os.Stderr, ethlog.TerminalFormat(false))
	ethlog.Root().SetHandler(ethlog.LvlFilterHandler(ethlog.LvlWarn, handler))

	if len(os.Args) < 2 {
		fatalf(usage)
	}
	switch os.Args[1] {
	case "generate":
		generateCommand(os.Args[2:])
	case "print":
		printCommand(os.Args[2:])
	default:
		fatalf(usage)
	}
}

// generateCommand generates a test chain.
func generateCommand(args []string) {
	var (
		cfg     generatorConfig
		outlist = flag.String("outputs", "", "Enabled output modules")
	)
	flag.IntVar(&cfg.chainLength, "length", 2, "The length of the pow chain to generate")
	// flag.IntVar(&cfg.posBlockCount, "poslength", 2, "The length of the pos chain to generate")
	flag.IntVar(&cfg.txInterval, "tx-interval", 10, "Add transactions to chain every n blocks")
	flag.IntVar(&cfg.txCount, "tx-count", 1, "Maximum number of txs per block")
	flag.IntVar(&cfg.forkInterval, "fork-interval", 0, "Number of blocks between fork activations")
	flag.StringVar(&cfg.outputDir, "outdir", ".", "Destination directory")
	flag.CommandLine.Parse(args)

	if *outlist != "" {
		cfg.outputs = splitAndTrim(*outlist)
	}

	cfg, err := cfg.withDefaults()
	if err != nil {
		panic(err)
	}
	g := newGenerator(cfg)
	if err := g.run(); err != nil {
		fatal(err)
	}
}

// printCommand displays the blocks in a chain.rlp file.
func printCommand(args []string) {
	var (
		verbose = flag.Bool("v", false, "If set, all block fields are displayed")
	)
	flag.CommandLine.Parse(args)
	if flag.NArg() != 1 {
		fatalf("Usage: hivechain print [ options ] <chain.rlp>")
	}

	file, err := os.Open(flag.Arg(0))
	if err != nil {
		fatal(err)
	}
	defer file.Close()

	s := rlp.NewStream(bufio.NewReader(file), 0)
	for i := 0; ; i++ {
		var block types.Block
		err := s.Decode(&block)
		if err == io.EOF {
			return
		} else if err != nil {
			fatalf("%d: %v", i, err)
		}
		if *verbose {
			js, _ := json.MarshalIndent(block.Header(), "", "  ")
			fmt.Printf("%d: %s\n", i, js)
		} else {
			fmt.Printf("%d: number %d, %x\n", i, block.Number(), block.Hash())
		}
	}
}

func splitAndTrim(s string) []string {
	var list []string
	for _, s := range strings.Split(s, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			list = append(list, s)
		}
	}
	return list
}

func fatalf(format string, args ...interface{}) {
	fatal(fmt.Errorf(format, args...))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
