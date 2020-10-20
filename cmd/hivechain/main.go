// The hivechain command assists with generating blockchain data for testing purposes.
//
// The 'generate' subcommand mines a new chain:
//
//     hivechain generate -length 10 -genesis ./genesis.json -blocktime 30 -output .
//
// The 'print' subcommand displays blocks in a chain.rlp file:
//
//     hivechain print -v chain.rlp
//
// The 'trim' subcommand extracts a range of blocks from a chain.rlp file:
//
//     hivechain trim -from 10 -to 100 chain.rlp newchain.rlp
//
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/ethereum/go-ethereum/core/types"
	ethlog "github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
)

const usage = "Usage: hivechain generate|print|trim [ options ] ..."

func main() {
	handler := ethlog.StreamHandler(os.Stderr, ethlog.TerminalFormat(false))
	ethlog.Root().SetHandler(ethlog.LvlFilterHandler(ethlog.LvlDebug, handler))

	if len(os.Args) < 2 {
		fatalf(usage)
	}
	switch os.Args[1] {
	case "generate":
		generateCommand(os.Args[2:])
	case "print":
		printCommand(os.Args[2:])
	case "trim":
		trimCommand(os.Args[2:])
	default:
		fatalf(usage)
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

// trimCommand exports a subset of chain.rlp to a new file.
func trimCommand(args []string) {
	var (
		from = flag.Uint("from", 0, "Start of block range to output")
		to   = flag.Uint("to", 0, "End of block range to output (0 = all blocks)")
	)
	flag.CommandLine.Parse(args)
	if flag.NArg() != 2 {
		fatalf("Usage: hivechain trim [ options ] <chain.rlp> <newchain.rlp>")
	}
	if *to > 0 && *to <= *from {
		fatalf("-to must be greater than -from")
	}

	input, err := os.Open(flag.Arg(0))
	if err != nil {
		fatal(err)
	}
	defer input.Close()

	output, err := os.OpenFile(flag.Arg(1), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		fatal(err)
	}
	defer output.Close()

	s := rlp.NewStream(bufio.NewReader(input), 0)
	written := 0
	for i := uint(0); ; i++ {
		data, err := s.Raw()
		if err == io.EOF {
			break
		} else if err != nil {
			fatalf("block %d: %v", i, err)
		}
		if i >= *from {
			if *to != 0 && i >= *to {
				break
			}
			output.Write(data)
			written++
		}
	}
	fmt.Println(written, "blocks written to", flag.Arg(1))
}

// generateCommand generates a test chain.
func generateCommand(args []string) {
	var (
		length    = flag.Uint("length", 2, "The length of the chain to generate")
		blocktime = flag.Uint("blocktime", 30, "The desired block time in seconds")
		genesis   = flag.String("genesis", "", "The path and filename to the source genesis.json")
		outdir    = flag.String("output", ".", "Chain destination folder")
	)
	flag.CommandLine.Parse(args)

	if *genesis == "" {
		fatalf("Missing -genesis option, please supply a genesis.json file.")
	}
	err := produceTestChainFromGenesisFile(*genesis, *outdir, *length, *blocktime)
	if err != nil {
		fatal(err)
	}
}

func fatalf(format string, args ...interface{}) {
	fatal(fmt.Errorf(format, args...))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
