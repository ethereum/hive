// The hq command queries and investigates hive test results.
//
// Subcommands:
//
//	hq clients                  # list known clients and aliases
//	hq runs [flags]             # list recent test runs
//	hq tests [flags] [file]     # list test cases in a run (-failed for failures only)
//	hq diff [flags] [file]      # colorized diff for failing tests
//	hq stats [flags]            # pass/fail rates across runs
//
// See README.md for details.
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	flag.Usage = usage

	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	args := os.Args[2:]
	switch os.Args[1] {
	case "clients":
		clientsCommand(args)
	case "runs":
		runsCommand(args)
	case "tests":
		testsCommand(args)
	case "diff":
		diffCommand(args)
	case "stats":
		statsCommand(args)
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(flag.CommandLine.Output(), "unknown subcommand %q\n\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	o := flag.CommandLine.Output()
	fmt.Fprintln(o, `Usage: hq <command> [flags] [args]

Commands:
  clients   List known clients and their aliases
  runs      List recent test runs
  tests     List test cases in a run (-failed for failures only)
  diff      Show colorized diff for failing tests
  stats     Show pass/fail rates across runs

Run "hq <command> -h" for command-specific flags.`)
}

func fatalf(format string, args ...interface{}) {
	fatal(fmt.Errorf(format, args...))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
